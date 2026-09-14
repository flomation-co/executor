package git_common

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	core "flomation.app/automate/executor"
	"github.com/go-git/go-git/v6"
	gossh "golang.org/x/crypto/ssh"
)

// Host key verification for git-over-SSH.
//
// go-git treats a nil HostKeyCallback as "build one from known_hosts", so
// without any of this every SSH git operation is verified against the RUNNER's
// ~/.ssh/known_hosts. On a runner that file is empty, which means a server
// nobody has manually seeded fails with "knownhosts: key is unknown" — an error
// that names neither the problem nor the fix, and that a flow author has no way
// to resolve from the editor.
//
// The fix is three things: let the author supply the key, let them knowingly
// skip the check, and say something useful when neither has happened.
//
// Skipping is offered but never the default. An attacker who can intercept
// git-over-SSH serves a MALICIOUS REPOSITORY, and flows routinely run what they
// clone through the makefile and script actions, so this is a supply-chain
// compromise rather than eavesdropping. (The private key itself is never
// exposed — public-key auth does not reveal it — but that is cold comfort.)

// Verification modes, reported on the action's output so an execution record
// shows how the connection was checked rather than leaving it to be inferred.
const (
	// HostKeyModeKnownHosts is the default: go-git checks the runner's
	// ~/.ssh/known_hosts.
	HostKeyModeKnownHosts = "known_hosts"
	// HostKeyModeSupplied means the action carried the expected key.
	HostKeyModeSupplied = "supplied"
	// HostKeyModeSkipped means verification was deliberately disabled.
	HostKeyModeSkipped = "skipped"
	// HostKeyModeNotApplicable covers HTTP, token and anonymous access.
	HostKeyModeNotApplicable = "not_applicable"
)

// HostKeyInputs documents the two inputs every SSH-capable git action carries.
//
// Reference only — each action re-declares them inline, because the manifest
// generator resolves literal composite literals and silently emits nothing for
// a cross-package var.
//
// The guidance lives in the LABEL rather than the placeholder on purpose: an AI
// agent reads an input's label as the parameter description and never sees the
// placeholder, and it was an agent that reported this gap in the first place.
var HostKeyInputs = [...]core.Connection{
	{
		Name:    "host_key",
		Type:    core.ConnectionTypeText,
		Label:   "Server Host Key — paste the output of `ssh-keyscan <host>`, or a SHA256:... fingerprint. Leave blank to use the runner's known_hosts.",
		Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"ssh"}},
	},
	{
		Name:    "skip_host_key_verification",
		Type:    core.ConnectionTypeBoolean,
		Label:   "Skip host key verification (INSECURE — an intercepted connection could serve a malicious repository)",
		Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"ssh"}},
	},
}

// HostKeyCallbackFromInputs builds the host key callback for an SSH connection.
//
// A nil callback means "leave go-git's default alone", which is the known_hosts
// path — so the secure default survives when neither input is set.
func HostKeyCallbackFromInputs(inputs []*core.Connection) (gossh.HostKeyCallback, string, error) {
	if skipHostKeyVerification(inputs) {
		return gossh.InsecureIgnoreHostKey(), HostKeyModeSkipped, nil
	}

	raw := optionalString("host_key", inputs)
	if raw == "" {
		return nil, HostKeyModeKnownHosts, nil
	}

	fingerprints, keys, err := ParseHostKeys(raw)
	if err != nil {
		return nil, "", err
	}
	return suppliedHostKeyCallback(fingerprints, keys), HostKeyModeSupplied, nil
}

// ParseHostKeys reads the three forms a person can actually get hold of:
//
//   - a known_hosts line, which is what `ssh-keyscan host` prints and what a
//     server's documentation usually publishes;
//   - a bare "keytype base64" pair, which is the same thing with the host
//     column stripped — a common way for it to arrive pasted out of a wiki;
//   - a SHA256:... fingerprint, which is what `ssh-keygen -lf` prints and what
//     an administrator will quote down the phone.
//
// Several lines may be given, since a host commonly offers more than one key
// type and ssh-keyscan prints all of them. Blank lines and # comments are
// ignored, so pasting keyscan output verbatim works.
func ParseHostKeys(raw string) (fingerprints []string, keys []gossh.PublicKey, err error) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "SHA256:") {
			fingerprints = append(fingerprints, line)
			continue
		}
		// MD5 fingerprints are still what some older tooling prints, and they
		// are not convertible to SHA-256 without the key itself — so say which
		// command produces the right thing rather than failing to parse.
		if looksLikeMD5Fingerprint(line) {
			return nil, nil, fmt.Errorf(
				"%q is an MD5 fingerprint, which cannot be checked against a modern host key — run `ssh-keyscan <host>` and paste that instead, or `ssh-keyscan <host> | ssh-keygen -lf -` for the SHA256: form", line)
		}

		// A bare "keytype base64" has no host column, which ParseKnownHosts
		// requires. Supplying a wildcard makes it parse; the host column is not
		// used for matching anyway (see suppliedHostKeyCallback).
		candidate := line
		if fields := strings.Fields(line); len(fields) >= 2 && isHostKeyType(fields[0]) {
			candidate = "* " + line
		}

		_, _, key, _, _, parseErr := gossh.ParseKnownHosts([]byte(candidate))
		if parseErr != nil {
			return nil, nil, fmt.Errorf(
				"could not read %q as a host key — expected the output of `ssh-keyscan <host>` (for example \"gitlab.example.com ssh-ed25519 AAAAC3Nz...\") or a SHA256:... fingerprint", truncate(line, 60))
		}
		keys = append(keys, key)
	}

	if len(fingerprints) == 0 && len(keys) == 0 {
		return nil, nil, fmt.Errorf("no host key was found in the Host Key input — paste the output of `ssh-keyscan <host>`, or a SHA256:... fingerprint")
	}
	return fingerprints, keys, nil
}

// suppliedHostKeyCallback accepts the server only if it presents one of the
// supplied keys.
//
// The host column of a known_hosts line is deliberately NOT matched. What the
// author is asserting by filling this in is "the server this action connects to
// presents this key", which is the same assertion a fingerprint makes, and
// matching hostnames as well would mean reimplementing known_hosts' pattern,
// port and hashed-entry syntax for no gain — the action already knows which
// host it is dialling.
func suppliedHostKeyCallback(fingerprints []string, keys []gossh.PublicKey) gossh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		presented := gossh.FingerprintSHA256(key)
		for _, f := range fingerprints {
			if f == presented {
				return nil
			}
		}

		marshalled := key.Marshal()
		for _, k := range keys {
			if bytes.Equal(k.Marshal(), marshalled) {
				return nil
			}
		}

		// This is the one that matters. Either the server's key was rotated, or
		// the connection is being intercepted, and the message has to leave both
		// on the table rather than nudging someone into pasting the new key.
		return fmt.Errorf(
			"host key mismatch for %s: the server presented %s (%s), which is not the Host Key configured on this action. "+
				"Either the server's key has genuinely changed — confirm the new one with whoever runs it — or this connection is being intercepted. Do not update the Host Key until you know which",
			hostname, presented, key.Type())
	}
}

// DescribeHostKeyError turns go-git's host key failures into something a flow
// author can act on without leaving the editor.
//
// The unmodified error is "ssh: handshake failed: knownhosts: key is unknown",
// which states neither what is wrong nor what to do, and is the reason this
// whole file exists. Any error that is not about host keys passes through
// untouched.
func DescribeHostKeyError(err error, repositoryURL string) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	host := HostFromRepositoryURL(repositoryURL)
	if host == "" {
		host = "<host>"
	}

	switch {
	case strings.Contains(message, "knownhosts: key is unknown"),
		strings.Contains(message, "knownhosts: key not found"):
		return fmt.Errorf(
			"the Git server's host key is not trusted by this runner, so the connection was refused before authentication. "+
				"Run `ssh-keyscan %s` and paste the output into this action's Host Key input. "+
				"If you cannot reach the server to do that, tick Skip host key verification — but that allows an intercepted connection to serve a malicious repository. (%w)",
			host, err)

	case strings.Contains(message, "knownhosts: key mismatch"):
		return fmt.Errorf(
			"the Git server at %s presented a host key that does not match the one this runner already trusts. "+
				"Either the server's key has genuinely changed, or the connection is being intercepted — confirm the current key with whoever runs the server before changing anything. (%w)",
			host, err)
	}
	return err
}

// HostFromRepositoryURL pulls the hostname out of a git remote so the error can
// name the exact ssh-keyscan command to run.
//
// Handles the two forms in the wild: scp-style (git@host:group/repo.git) and
// URL-style (ssh://git@host:2222/group/repo.git). Returns "" when it cannot
// tell, and the caller falls back to a placeholder rather than guessing.
func HostFromRepositoryURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if idx := strings.Index(raw, "://"); idx >= 0 {
		rest := raw[idx+3:]
		if at := strings.LastIndex(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		if slash := strings.IndexAny(rest, "/"); slash >= 0 {
			rest = rest[:slash]
		}
		// A port is not part of the hostname ssh-keyscan wants by default, and
		// including it would make the suggested command wrong.
		if host, _, err := net.SplitHostPort(rest); err == nil {
			return host
		}
		return rest
	}

	// scp-style: everything between an optional user@ and the first colon.
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		raw = raw[at+1:]
	}
	if colon := strings.Index(raw, ":"); colon >= 0 {
		raw = raw[:colon]
	}
	if strings.ContainsAny(raw, "/ ") {
		return ""
	}
	return raw
}

func skipHostKeyVerification(inputs []*core.Connection) bool {
	c := core.FindConnection("skip_host_key_verification", inputs)
	if c == nil {
		return false
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	// The editor stores some boolean values as strings.
	if s := c.String(); s != nil {
		return strings.EqualFold(strings.TrimSpace(*s), "true")
	}
	return false
}

func optionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	v := strings.TrimSpace(*c.String())
	// An unresolved reference is not a value; treating "${secrets.Missing}" as
	// a host key would fail as a parse error rather than as a missing secret.
	if strings.HasPrefix(v, "${") {
		return ""
	}
	return v
}

func isHostKeyType(field string) bool {
	switch {
	case field == "ssh-rsa", field == "ssh-dss", field == "ssh-ed25519":
		return true
	case strings.HasPrefix(field, "ecdsa-sha2-"):
		return true
	case strings.HasPrefix(field, "sk-"): // FIDO-backed keys
		return true
	}
	return false
}

// looksLikeMD5Fingerprint matches the colon-separated hex form, e.g.
// "16:27:ac:a5:76:28:2d:36:63:1b:56:4d:eb:df:a6:48".
func looksLikeMD5Fingerprint(line string) bool {
	parts := strings.Split(strings.TrimPrefix(line, "MD5:"), ":")
	if len(parts) != 16 {
		return false
	}
	for _, p := range parts {
		if len(p) != 2 {
			return false
		}
		for _, r := range p {
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// RemoteURL returns the configured URL of a remote, used only to name the host
// in an error message. Pull and push take a local path rather than a URL, so
// without this their host key errors could not say which server to keyscan.
// Returns "" when the remote cannot be read; the caller falls back to a
// placeholder.
func RemoteURL(repo *git.Repository, remoteName string) string {
	if repo == nil {
		return ""
	}
	if remoteName == "" {
		remoteName = "origin"
	}
	remote, err := repo.Remote(remoteName)
	if err != nil || remote == nil {
		return ""
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}
