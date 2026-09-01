package filetransfer_upload

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Upload File (FTP/SFTP)"
	Description  = "Upload a file to a remote FTP, FTPS, or SFTP server"
	Website      = "https://www.flomation.co"
	Icon         = "server+arrow-up"
	Date         = "20/06/2026"
	Type         = core.ActionTypeAction
)

// dialTimeout bounds the TCP connect / handshake for every protocol. The
// upload itself is not separately bounded — a large file legitimately takes
// time, and the surrounding flow already has its own deadline.
const dialTimeout = 30 * time.Second

var Inputs = [...]core.Connection{
	{
		Name:     "protocol",
		Type:     core.ConnectionTypeString,
		Label:    "Protocol",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "SFTP (over SSH)", Value: "sftp"},
			{Name: "FTP (plain)", Value: "ftp"},
			{Name: "FTPS (FTP over TLS)", Value: "ftps"},
		},
	},
	{
		Name:        "host",
		Type:        core.ConnectionTypeString,
		Label:       "Host",
		Placeholder: "files.example.com or 10.0.0.5",
		Required:    true,
	},
	{
		Name:        "port",
		Type:        core.ConnectionTypeInteger,
		Label:       "Port",
		Placeholder: "Defaults to 22 for SFTP, 21 for FTP/FTPS",
	},
	{
		Name:        "username",
		Type:        core.ConnectionTypeString,
		Label:       "Username",
		Placeholder: "ftpuser",
		Required:    true,
	},
	{
		Name:     "auth_method",
		Type:     core.ConnectionTypeString,
		Label:    "Authentication",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Password", Value: "password"},
			{Name: "Private Key (SFTP only)", Value: "key"},
		},
	},
	{
		Name:        "password",
		Type:        core.ConnectionTypeSecret,
		Label:       "Password",
		Placeholder: "Select an environment secret",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"password"}},
	},
	{
		Name:        "private_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "SSH Private Key",
		Placeholder: "Secret holding the private key (SFTP only)",
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
		Name:        "host_fingerprint",
		Type:        core.ConnectionTypeString,
		Label:       "SFTP Host Key Fingerprint",
		Placeholder: "SHA256:... (optional — verifies the SSH host key)",
		Visible:     &core.VisibleWhen{Field: "protocol", Values: []string{"sftp"}},
	},
	{
		Name:        "tls_skip_verify",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Skip TLS Certificate Verification",
		Placeholder: "Allow self-signed certificates (FTPS only — use with care)",
		Visible:     &core.VisibleWhen{Field: "protocol", Values: []string{"ftps"}},
	},
	{
		Name:        "remote_path",
		Type:        core.ConnectionTypeString,
		Label:       "Remote Path",
		Placeholder: "/uploads/report.pdf (full path including filename)",
		Required:    true,
	},
	{
		Name:        "content",
		Type:        core.ConnectionTypeText,
		Label:       "File Content",
		Placeholder: "Text to write, or base64 if 'Content is Base64' is on",
		Required:    true,
	},
	{
		Name:        "content_is_base64",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Content is Base64 (binary files)",
		Placeholder: "Decode the content before uploading — for PDFs, images, etc.",
	},
	{
		Name:        "create_dirs",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Create Missing Directories",
		Placeholder: "Create the parent path on the server if it doesn't exist",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "remote_path", Type: core.ConnectionTypeString, Label: "Remote Path"},
	{Name: "bytes_sent", Type: core.ConnectionTypeInteger, Label: "Bytes Sent"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	protocol := strings.ToLower(strings.TrimSpace(strVal(core.FindConnection("protocol", inputs))))
	if protocol == "" {
		return nil, fmt.Errorf("protocol is required (sftp, ftp, or ftps)")
	}

	host := strings.TrimSpace(strVal(core.FindConnection("host", inputs)))
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}

	username := strings.TrimSpace(strVal(core.FindConnection("username", inputs)))
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	// remote_path is the destination. We don't trim path traversal here the way
	// the local file/write action does — a remote path is the user's own server
	// and ".." may be legitimate — but it has to name something.
	remotePath := strings.TrimSpace(strVal(core.FindConnection("remote_path", inputs)))

	data, mimeType, err := resolveContent(flow, inputs)
	if err != nil {
		return nil, err
	}

	// A path ending in "/" names the directory, so keep the file's own name
	// rather than trying to write to the directory itself.
	remotePath = core.UploadDestination(remotePath, contentValue(inputs), mimeType, "upload")
	if remotePath == "" {
		return nil, fmt.Errorf("remote_path is required")
	}

	createDirs := boolVal(core.FindConnection("create_dirs", inputs))
	port := resolvePort(inputs, protocol)
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	var summary string
	switch protocol {
	case "sftp":
		summary, err = uploadSFTP(flow, inputs, addr, username, remotePath, data, createDirs)
	case "ftp", "ftps":
		summary, err = uploadFTP(flow, inputs, protocol, addr, username, remotePath, data, createDirs)
	default:
		return nil, fmt.Errorf("unknown protocol %q (expected sftp, ftp, or ftps)", protocol)
	}
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": summary,
		"remote_path": remotePath,
		"bytes_sent":  int64(len(data)),
		"success":     true,
	}, nil
}

// resolveContent reads the content input and, when content_is_base64 is set,
// decodes it so binary payloads (PDFs, images) round-trip intact. Raw text is
// passed through byte-for-byte.
// It also returns the MIME type when the content came from a reference, so a
// derived filename can be given the right extension.
func resolveContent(flow *core.Flow, inputs []*core.Connection) ([]byte, string, error) {
	conn := core.FindConnection("content", inputs)
	if conn == nil || conn.String() == nil {
		return nil, "", fmt.Errorf("content is required")
	}
	raw := *conn.String()

	// A workspace file or blob reference (e.g. a large media action output)
	// takes precedence over the base64/text handling.
	if core.IsFileRef(raw) || core.IsBlobToken(raw) {
		data, mimeType, err := flow.ResolveToBytes(raw)
		if err != nil {
			return nil, "", fmt.Errorf("read content reference: %w", err)
		}
		return data, mimeType, nil
	}

	if boolVal(core.FindConnection("content_is_base64", inputs)) {
		// Tolerate whitespace/newlines that often creep into pasted base64.
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
		if err != nil {
			return nil, "", fmt.Errorf("content is not valid base64: %w", err)
		}
		return decoded, "", nil
	}
	return []byte(raw), "", nil
}

// resolvePort returns the user-supplied port, or the protocol default
// (22 for SFTP, 21 for FTP/FTPS) when none is given.
func resolvePort(inputs []*core.Connection, protocol string) int64 {
	if p := core.FindConnection("port", inputs); p != nil {
		if n := p.Number(); n != nil && *n > 0 {
			return *n
		}
	}
	if protocol == "sftp" {
		return 22
	}
	return 21
}

// uploadSFTP opens an SSH connection, layers an SFTP client on top, and writes
// the payload to remotePath. It mirrors the auth and host-key handling of the
// ssh/run action.
func uploadSFTP(flow *core.Flow, inputs []*core.Connection, addr, username, remotePath string, data []byte, createDirs bool) (string, error) {
	authMethod, err := buildSSHAuth(inputs)
	if err != nil {
		return "", err
	}

	fingerprint := strings.TrimSpace(strVal(core.FindConnection("host_fingerprint", inputs)))
	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if fingerprint != "" {
		hostKeyCallback = fingerprintCallback(fingerprint)
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: hostKeyCallback,
		Timeout:         dialTimeout,
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return "", fmt.Errorf("failed to open SFTP session: %w", err)
	}
	defer sftpClient.Close()

	if createDirs {
		if dir := path.Dir(remotePath); dir != "" && dir != "." {
			if err := sftpClient.MkdirAll(dir); err != nil {
				return "", fmt.Errorf("failed to create remote directory %s: %w", dir, err)
			}
		}
	}

	f, err := sftpClient.Create(remotePath)
	if err != nil {
		return "", fmt.Errorf("failed to create remote file %s: %w", remotePath, err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return "", fmt.Errorf("failed to write remote file %s: %w", remotePath, err)
	}

	summary := fmt.Sprintf("Uploaded %d bytes to %s via SFTP", len(data), remotePath)
	if fingerprint == "" {
		summary = "Warning: SFTP host key not verified (no fingerprint supplied). " + summary
	}
	return summary, nil
}

// uploadFTP connects over FTP or, when protocol is "ftps", FTP with explicit
// TLS (the common AUTH TLS form on port 21), logs in with a username and
// password, and stores the payload at remotePath.
func uploadFTP(flow *core.Flow, inputs []*core.Connection, protocol, addr, username, remotePath string, data []byte, createDirs bool) (string, error) {
	if method := strVal(core.FindConnection("auth_method", inputs)); method == "key" {
		return "", fmt.Errorf("private key authentication is only supported for SFTP; use Password for FTP/FTPS")
	}
	password := strVal(core.FindConnection("password", inputs))
	if password == "" {
		return "", fmt.Errorf("password is required for FTP/FTPS authentication")
	}

	opts := []ftp.DialOption{
		ftp.DialWithContext(flow.GoContext()),
		ftp.DialWithTimeout(dialTimeout),
	}
	if protocol == "ftps" {
		host, _, _ := net.SplitHostPort(addr)
		tlsConfig := &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: boolVal(core.FindConnection("tls_skip_verify", inputs)),
		}
		opts = append(opts, ftp.DialWithExplicitTLS(tlsConfig))
	}

	conn, err := ftp.Dial(addr, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer func() { _ = conn.Quit() }()

	if err := conn.Login(username, password); err != nil {
		return "", fmt.Errorf("FTP login failed: %w", err)
	}

	if createDirs {
		if dir := path.Dir(remotePath); dir != "" && dir != "." {
			makeFTPDirs(conn, dir)
		}
	}

	if err := conn.Stor(remotePath, strings.NewReader(string(data))); err != nil {
		return "", fmt.Errorf("failed to store remote file %s: %w", remotePath, err)
	}

	label := "FTP"
	if protocol == "ftps" {
		label = "FTPS"
	}
	return fmt.Sprintf("Uploaded %d bytes to %s via %s", len(data), remotePath, label), nil
}

// makeFTPDirs walks the directory components of dir and creates each in turn.
// MakeDir errors are ignored because the usual cause is "directory already
// exists", which is not a failure for our purposes; a genuinely un-creatable
// path surfaces later as a Stor error.
func makeFTPDirs(conn *ftp.ServerConn, dir string) {
	parts := strings.Split(strings.Trim(dir, "/"), "/")
	cur := ""
	if strings.HasPrefix(dir, "/") {
		cur = "/"
	}
	for _, p := range parts {
		if p == "" {
			continue
		}
		if cur == "" || cur == "/" {
			cur += p
		} else {
			cur += "/" + p
		}
		_ = conn.MakeDir(cur)
	}
}

// fingerprintCallback verifies the SSH server's host key against a user-supplied
// SHA-256 fingerprint (the "SHA256:..." form from `ssh-keygen -lf`). A mismatch
// aborts the connection, guarding against MITM for callers who supply one.
func fingerprintCallback(expected string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		got := ssh.FingerprintSHA256(key)
		if got != expected {
			return fmt.Errorf("host key mismatch for %s: got %s, expected %s", hostname, got, expected)
		}
		return nil
	}
}

// buildSSHAuth resolves the SFTP authentication method. "password" uses a
// literal password; "key" parses a private key, decrypting it with the
// passphrase when one is supplied.
func buildSSHAuth(inputs []*core.Connection) (ssh.AuthMethod, error) {
	method := strVal(core.FindConnection("auth_method", inputs))
	if method == "" {
		method = "password"
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
// secrets and PEM whitespace are preserved exactly).
func strVal(c *core.Connection) string {
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

// boolVal returns the boolean value of a connection, defaulting to false.
func boolVal(c *core.Connection) bool {
	if c == nil || c.Boolean() == nil {
		return false
	}
	return *c.Boolean()
}

// contentValue returns the raw content input, so the destination resolver can
// read the filename off a flo:blob:/flo:file: reference.
func contentValue(inputs []*core.Connection) string {
	conn := core.FindConnection("content", inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}
