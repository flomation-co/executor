package ssh_run

import (
	"bytes"
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
		Name:        "username",
		Type:        core.ConnectionTypeString,
		Label:       "Username",
		Placeholder: "root",
		Required:    true,
	},
	{
		Name:  "auth_method",
		Type:  core.ConnectionTypeString,
		Label: "Authentication",
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

	command := strVal(core.FindConnection("command", inputs))
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	port := int64(22)
	if p := core.FindConnection("port", inputs); p != nil {
		if n := p.Number(); n != nil && *n > 0 {
			port = *n
		}
	}

	authMethod, err := buildAuth(inputs)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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

	// Run the command. A non-zero exit status is not treated as a fatal
	// error — it's surfaced through the exit_code output so downstream
	// nodes can branch on it, mirroring how a shell would behave.
	exitCode := int64(0)
	runErr := session.Run(command)
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

	return map[string]interface{}{
		"tool_result": summary,
		"stdout":      stdout.String(),
		"stderr":      stderr.String(),
		"exit_code":   exitCode,
	}, nil
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
