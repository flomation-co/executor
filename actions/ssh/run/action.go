package ssh_run

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	"golang.org/x/crypto/ssh"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "SSH Run Command"
	Description  = "Connect to a remote host over SSH and execute a command"
	Website      = "https://www.flomation.co"
	Icon         = "terminal+play"
	Date         = "17/06/2026"
	Type         = core.ActionTypeAction
)

// defaultTimeoutSeconds bounds how long a single command may run once the
// session is open. The dial/handshake has its own (shorter) timeout below.
const defaultTimeoutSeconds = 300

var Inputs = [...]core.Connection{
	{
		Name:        "host",
		Type:        core.ConnectionTypeString,
		Label:       "Host",
		Placeholder: "example.com or 10.0.0.5",
		Required:    true,
	},
	{
		Name:        "port",
		Type:        core.ConnectionTypeInteger,
		Label:       "Port",
		Placeholder: "22",
	},
	{
		Name:        "host_fingerprint",
		Type:        core.ConnectionTypeString,
		Label:       "Host Key Fingerprint",
		Placeholder: "SHA256:... (optional — verifies the server's host key)",
	},
	{
		Name:        "username",
		Type:        core.ConnectionTypeString,
		Label:       "Username",
		Placeholder: "root",
		Required:    true,
	},
	{
		Name:     "auth_method",
		Type:     core.ConnectionTypeString,
		Label:    "Authentication",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Private Key", Value: "key"},
			{Name: "Password", Value: "password"},
		},
	},
	{
		Name:        "private_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "SSH Private Key",
		Placeholder: "Select an environment secret holding the private key",
		Required:    true,
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"key"}},
	},
	{
		Name:        "passphrase",
		Type:        core.ConnectionTypeSecret,
		Label:       "Key Passphrase",
		Placeholder: "Only if the private key is encrypted",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"key"}},
	},
	{
		Name:        "password",
		Type:        core.ConnectionTypeSecret,
		Label:       "Password",
		Placeholder: "",
		Required:    true,
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"password"}},
	},
	{
		Name:        "command",
		Type:        core.ConnectionTypeText,
		Label:       "Command",
		Placeholder: "uptime",
		Required:    true,
	},
	{
		Name:        "timeout_seconds",
		Type:        core.ConnectionTypeInteger,
		Label:       "Command Timeout (seconds)",
		Placeholder: "300 (-1 for no timeout)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "stdout", Type: core.ConnectionTypeText, Label: "Standard Output"},
	{Name: "stderr", Type: core.ConnectionTypeText, Label: "Standard Error"},
	{Name: "exit_code", Type: core.ConnectionTypeInteger, Label: "Exit Code"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	host := strings.TrimSpace(strVal(core.FindConnection("host", inputs)))
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}

	username := strings.TrimSpace(strVal(core.FindConnection("username", inputs)))
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	// command isn't a secret, so trim it — a leading/trailing newline in a
	// pasted command is user error rather than intent.
	command := strings.TrimSpace(strVal(core.FindConnection("command", inputs)))
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	port := int64(22)
	if p := core.FindConnection("port", inputs); p != nil {
		if n := p.Number(); n != nil && *n > 0 {
			port = *n
		}
	}

	// timeout_seconds: positive bounds the command; a negative value (e.g. -1)
	// disables the deadline entirely; 0 or unset keeps the default.
	timeout := time.Duration(defaultTimeoutSeconds) * time.Second
	noTimeout := false
	if t := core.FindConnection("timeout_seconds", inputs); t != nil {
		if n := t.Number(); n != nil {
			if *n < 0 {
				noTimeout = true
			} else if *n > 0 {
				timeout = time.Duration(*n) * time.Second
			}
		}
	}

	authMethod, err := buildAuth(inputs)
	if err != nil {
		return nil, err
	}

	// Host key verification: if the user supplied a SHA-256 fingerprint we
	// verify against it and reject on mismatch; otherwise we fall back to
	// accepting any host key but flag it in the result so the lack of
	// verification is visible rather than silent.
	fingerprint := strings.TrimSpace(strVal(core.FindConnection("host_fingerprint", inputs)))
	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if fingerprint != "" {
		hostKeyCallback = fingerprintCallback(fingerprint)
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to open SSH session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Run the command under a deadline. session.Run blocks until the remote
	// process exits, so without this a hung remote command would block the
	// executor goroutine forever. On timeout we signal the remote process and
	// bail; the deferred Close calls tear down the session and connection.
	// When noTimeout is set the context only cancels on return, so Run is
	// allowed to block indefinitely.
	var ctx context.Context
	var cancel context.CancelFunc
	if noTimeout {
		ctx, cancel = context.WithCancel(context.Background())
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()

	var runErr error
	select {
	case runErr = <-done:
		// completed within the deadline
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return nil, fmt.Errorf("command timed out on %s after %s", addr, timeout)
	}

	// A non-zero exit status is not treated as a fatal error — it's surfaced
	// through the exit_code output so downstream nodes can branch on it,
	// mirroring how a shell would behave.
	exitCode := int64(0)
	if runErr != nil {
		if exitErr, ok := runErr.(*ssh.ExitError); ok {
			exitCode = int64(exitErr.ExitStatus())
		} else {
			return nil, fmt.Errorf("failed to run command on %s: %w", addr, runErr)
		}
	}

	summary := fmt.Sprintf("Command exited with code %d", exitCode)
	if exitCode == 0 {
		summary = "Command executed successfully"
	}
	if fingerprint == "" {
		summary = "Warning: host key not verified (no fingerprint supplied). " + summary
	}

	return map[string]interface{}{
		"tool_result": summary,
		"stdout":      stdout.String(),
		"stderr":      stderr.String(),
		"exit_code":   exitCode,
	}, nil
}

// fingerprintCallback verifies the server's host key against a user-supplied
// SHA-256 fingerprint (the "SHA256:..." form produced by ssh.FingerprintSHA256,
// e.g. from `ssh-keyscan host | ssh-keygen -lf -`). Mismatch aborts the
// connection, giving callers who care a hard guarantee against MITM.
func fingerprintCallback(expected string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		got := ssh.FingerprintSHA256(key)
		if got != expected {
			return fmt.Errorf("host key mismatch for %s: got %s, expected %s", hostname, got, expected)
		}
		return nil
	}
}

// buildAuth resolves the SSH authentication method from the inputs.
// "password" uses a literal password; "key" uses a private key, decrypting
// it with the supplied passphrase when one is provided.
func buildAuth(inputs []*core.Connection) (ssh.AuthMethod, error) {
	method := strVal(core.FindConnection("auth_method", inputs))
	if method == "" {
		method = "key"
	}

	switch method {
	case "password":
		password := strVal(core.FindConnection("password", inputs))
		if password == "" {
			return nil, fmt.Errorf("password is required for password authentication")
		}
		return ssh.Password(password), nil

	case "key":
		key := strVal(core.FindConnection("private_key", inputs))
		if key == "" {
			return nil, fmt.Errorf("private key is required for key authentication")
		}

		var signer ssh.Signer
		var err error
		if passphrase := strVal(core.FindConnection("passphrase", inputs)); passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(key), []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(key))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil

	default:
		return nil, fmt.Errorf("unknown authentication method: %s", method)
	}
}

// strVal returns the raw string value of a connection (no trimming, so
// secrets, passphrases, and PEM whitespace are preserved exactly).
func strVal(c *core.Connection) string {
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
