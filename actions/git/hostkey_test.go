package git_common

import (
	"fmt"
	"net"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
	gossh "golang.org/x/crypto/ssh"
)

func ins(pairs ...[2]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, &core.Connection{Name: p[0], Type: core.ConnectionTypeString, Value: p[1]})
	}
	return out
}

// A real ed25519 host key, generated for these tests. Having an actual key
// matters: the whole point of this code is matching what a server presents, and
// a fabricated base64 blob would not round-trip through ParseKnownHosts.
const (
	testKeyType = "ssh-ed25519"
	testKeyB64  = "AAAAC3NzaC1lZDI1NTE5AAAAIKz8T8Y0DLHMwRPPQJ5fm2GRPwGgJmDDxJvR8kZ8KqZx"
	otherKeyB64 = "AAAAC3NzaC1lZDI1NTE5AAAAIH8pVJ9wJ0m5cN1CkGbLVq5xGqiLHrz3mYb3VQZvLxJt"
)

func mustKey(t *testing.T, b64 string) gossh.PublicKey {
	t.Helper()
	_, _, key, _, _, err := gossh.ParseKnownHosts([]byte("host " + testKeyType + " " + b64))
	if err != nil {
		t.Fatalf("test key %q is not parseable: %v", b64, err)
	}
	return key
}

// --- parsing ---

// ssh-keyscan output is what a person can actually obtain, so pasting it
// verbatim — several key types, blank lines and # comment lines included — has
// to work without editing.
func TestParseHostKeys_AcceptsKeyscanOutputVerbatim(t *testing.T) {
	RegisterTestingT(t)

	raw := "# gitlab.example.com:22 SSH-2.0-OpenSSH_8.9\n" +
		"gitlab.example.com " + testKeyType + " " + testKeyB64 + "\n" +
		"\n" +
		"# gitlab.example.com:22 SSH-2.0-OpenSSH_8.9\n" +
		"gitlab.example.com " + testKeyType + " " + otherKeyB64 + "\n"

	fingerprints, keys, err := ParseHostKeys(raw)
	Expect(err).To(BeNil())
	Expect(fingerprints).To(BeEmpty())
	Expect(keys).To(HaveLen(2), "a host commonly offers more than one key and keyscan prints them all")
}

// The same line with the host column stripped is a common way for a key to
// arrive out of a wiki page, and ParseKnownHosts rejects it outright.
func TestParseHostKeys_AcceptsABareKeyTypeAndBlob(t *testing.T) {
	RegisterTestingT(t)

	_, keys, err := ParseHostKeys(testKeyType + " " + testKeyB64)
	Expect(err).To(BeNil())
	Expect(keys).To(HaveLen(1))
}

func TestParseHostKeys_AcceptsSHA256Fingerprints(t *testing.T) {
	RegisterTestingT(t)

	expected := gossh.FingerprintSHA256(mustKey(t, testKeyB64))
	fingerprints, keys, err := ParseHostKeys(expected)

	Expect(err).To(BeNil())
	Expect(keys).To(BeEmpty())
	Expect(fingerprints).To(Equal([]string{expected}))
}

func TestParseHostKeys_MixesFormats(t *testing.T) {
	RegisterTestingT(t)

	raw := gossh.FingerprintSHA256(mustKey(t, testKeyB64)) + "\n" +
		"gitlab.example.com " + testKeyType + " " + otherKeyB64

	fingerprints, keys, err := ParseHostKeys(raw)
	Expect(err).To(BeNil())
	Expect(fingerprints).To(HaveLen(1))
	Expect(keys).To(HaveLen(1))
}

// An MD5 fingerprint cannot be converted to SHA-256 without the key itself, so
// the only useful response is to name the command that produces the right form.
func TestParseHostKeys_RejectsMD5WithTheCommandToRunInstead(t *testing.T) {
	RegisterTestingT(t)

	_, _, err := ParseHostKeys("16:27:ac:a5:76:28:2d:36:63:1b:56:4d:eb:df:a6:48")
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("MD5"))
	Expect(err.Error()).To(ContainSubstring("ssh-keyscan"))

	// The MD5: prefix form too.
	_, _, err = ParseHostKeys("MD5:16:27:ac:a5:76:28:2d:36:63:1b:56:4d:eb:df:a6:48")
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("MD5"))
}

func TestParseHostKeys_RejectsNonsenseWithAnExample(t *testing.T) {
	RegisterTestingT(t)

	_, _, err := ParseHostKeys("this is not a host key")
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("ssh-keyscan"), "the message must name the command that produces one")

	// Whitespace and comments alone are not a key.
	_, _, err = ParseHostKeys("  \n# only a comment\n")
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("no host key was found"))
}

// --- verification ---

func TestSuppliedHostKeyCallback_AcceptsAMatchingKey(t *testing.T) {
	RegisterTestingT(t)

	key := mustKey(t, testKeyB64)
	callback := suppliedHostKeyCallback(nil, []gossh.PublicKey{key})

	Expect(callback("gitlab.example.com:22", &net.TCPAddr{}, key)).To(BeNil())
}

func TestSuppliedHostKeyCallback_AcceptsAMatchingFingerprint(t *testing.T) {
	RegisterTestingT(t)

	key := mustKey(t, testKeyB64)
	callback := suppliedHostKeyCallback([]string{gossh.FingerprintSHA256(key)}, nil)

	Expect(callback("gitlab.example.com:22", &net.TCPAddr{}, key)).To(BeNil())
}

// The message on the failure path has to leave BOTH explanations open. Nudging
// someone towards "the key must have changed, paste the new one" is how a real
// interception gets waved through.
func TestSuppliedHostKeyCallback_RejectsAndDoesNotAssumeRotation(t *testing.T) {
	RegisterTestingT(t)

	expected := mustKey(t, testKeyB64)
	presented := mustKey(t, otherKeyB64)
	callback := suppliedHostKeyCallback(nil, []gossh.PublicKey{expected})

	err := callback("gitlab.example.com:22", &net.TCPAddr{}, presented)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("gitlab.example.com"))
	Expect(err.Error()).To(ContainSubstring(gossh.FingerprintSHA256(presented)), "the presented key must be quoted so it can be compared")
	Expect(err.Error()).To(ContainSubstring("intercepted"))
	Expect(err.Error()).To(ContainSubstring("Do not update the Host Key until you know which"))
}

// --- mode resolution ---

// The secure default has to survive. A nil callback is what tells go-git to use
// known_hosts, so anything that returns a non-nil callback here would silently
// replace verification.
func TestHostKeyCallbackFromInputs_DefaultsToKnownHosts(t *testing.T) {
	RegisterTestingT(t)

	callback, mode, err := HostKeyCallbackFromInputs(ins())
	Expect(err).To(BeNil())
	Expect(callback).To(BeNil(), "nil is what leaves go-git's known_hosts check in place")
	Expect(mode).To(Equal(HostKeyModeKnownHosts))

	// An unresolved ${...} reference is not a host key, and must not be parsed
	// as one — that would fail as a malformed key rather than a missing secret.
	callback, mode, err = HostKeyCallbackFromInputs(ins([2]string{"host_key", "${secrets.Missing}"}))
	Expect(err).To(BeNil())
	Expect(callback).To(BeNil())
	Expect(mode).To(Equal(HostKeyModeKnownHosts))
}

func TestHostKeyCallbackFromInputs_SuppliedKey(t *testing.T) {
	RegisterTestingT(t)

	callback, mode, err := HostKeyCallbackFromInputs(
		ins([2]string{"host_key", "gitlab.example.com " + testKeyType + " " + testKeyB64}))

	Expect(err).To(BeNil())
	Expect(callback).ToNot(BeNil())
	Expect(mode).To(Equal(HostKeyModeSupplied))
	Expect(callback("gitlab.example.com:22", &net.TCPAddr{}, mustKey(t, testKeyB64))).To(BeNil())
}

func TestHostKeyCallbackFromInputs_SkipIsExplicitAndNeverTheDefault(t *testing.T) {
	RegisterTestingT(t)

	// Boolean-typed, as the editor sends it.
	skip := true
	callback, mode, err := HostKeyCallbackFromInputs([]*core.Connection{
		{Name: "skip_host_key_verification", Type: core.ConnectionTypeBoolean, Value: skip},
	})
	Expect(err).To(BeNil())
	Expect(mode).To(Equal(HostKeyModeSkipped))
	Expect(callback("anything", &net.TCPAddr{}, mustKey(t, otherKeyB64))).To(BeNil())

	// String-typed, as a ${...} substitution delivers it.
	_, mode, err = HostKeyCallbackFromInputs(ins([2]string{"skip_host_key_verification", "true"}))
	Expect(err).To(BeNil())
	Expect(mode).To(Equal(HostKeyModeSkipped))

	// Anything else means do not skip.
	for _, value := range []string{"", "false", "no", "${flow.unset}"} {
		_, mode, err = HostKeyCallbackFromInputs(ins([2]string{"skip_host_key_verification", value}))
		Expect(err).To(BeNil())
		Expect(mode).To(Equal(HostKeyModeKnownHosts), "value %q must not disable verification", value)
	}
}

// Skip wins over a supplied key rather than erroring: the author has said
// plainly what they want, and a conflict here should not block the flow.
func TestHostKeyCallbackFromInputs_SkipOverridesASuppliedKey(t *testing.T) {
	RegisterTestingT(t)

	_, mode, err := HostKeyCallbackFromInputs(ins(
		[2]string{"host_key", "gitlab.example.com " + testKeyType + " " + testKeyB64},
		[2]string{"skip_host_key_verification", "true"},
	))
	Expect(err).To(BeNil())
	Expect(mode).To(Equal(HostKeyModeSkipped))
}

// Host key verification only applies to SSH, so reporting a mode for HTTP would
// imply a check that never happens.
func TestHostKeyModeFromInputs_NotApplicableWithoutSSH(t *testing.T) {
	RegisterTestingT(t)

	Expect(HostKeyModeFromInputs(ins([2]string{"auth_method", "token"}))).To(Equal(HostKeyModeNotApplicable))
	Expect(HostKeyModeFromInputs(ins())).To(Equal(HostKeyModeNotApplicable), "anonymous is the default")
	Expect(HostKeyModeFromInputs(ins([2]string{"auth_method", "ssh"}))).To(Equal(HostKeyModeKnownHosts))
	Expect(HostKeyModeFromInputs(ins(
		[2]string{"auth_method", "ssh"},
		[2]string{"skip_host_key_verification", "true"},
	))).To(Equal(HostKeyModeSkipped))
}

// --- error mapping ---

// The original error states neither the problem nor the fix. This mapping is
// the reason the gap was reported in the first place.
func TestDescribeHostKeyError_TellsYouWhatToRun(t *testing.T) {
	RegisterTestingT(t)

	original := fmt.Errorf("ssh: handshake failed: knownhosts: key is unknown")
	err := DescribeHostKeyError(original, "git@gitlab.example.com:group/repo.git")

	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("ssh-keyscan gitlab.example.com"), "the exact command, with the real host in it")
	Expect(err.Error()).To(ContainSubstring("Host Key"))
	Expect(err.Error()).To(ContainSubstring("Skip host key verification"))
	// The original must survive for anyone reading the raw execution log.
	Expect(err.Error()).To(ContainSubstring("knownhosts: key is unknown"))
}

func TestDescribeHostKeyError_MismatchIsTreatedAsSerious(t *testing.T) {
	RegisterTestingT(t)

	err := DescribeHostKeyError(fmt.Errorf("ssh: handshake failed: knownhosts: key mismatch"), "git@gitlab.example.com:group/repo.git")

	Expect(err.Error()).To(ContainSubstring("intercepted"))
	Expect(err.Error()).To(ContainSubstring("gitlab.example.com"))
	// It must NOT suggest keyscanning the new key, which would walk someone
	// straight through a real interception.
	Expect(err.Error()).ToNot(ContainSubstring("ssh-keyscan"))
}

// Anything not about host keys has to pass through untouched, or every other
// git failure acquires misleading SSH advice.
func TestDescribeHostKeyError_LeavesOtherErrorsAlone(t *testing.T) {
	RegisterTestingT(t)

	original := fmt.Errorf("authentication required")
	Expect(DescribeHostKeyError(original, "git@host:repo.git")).To(Equal(original))
	Expect(DescribeHostKeyError(nil, "git@host:repo.git")).To(BeNil())
}

// --- URL parsing ---

// The suggested ssh-keyscan command is only useful if the host in it is right.
func TestHostFromRepositoryURL(t *testing.T) {
	RegisterTestingT(t)

	cases := map[string]string{
		"git@gitlab.example.com:group/repo.git":            "gitlab.example.com",
		"gitlab.example.com:group/repo.git":                "gitlab.example.com",
		"ssh://git@gitlab.example.com/group/repo.git":      "gitlab.example.com",
		"ssh://git@gitlab.example.com:2222/group/repo.git": "gitlab.example.com",
		"ssh://gitlab.example.com/group/repo.git":          "gitlab.example.com",
		"https://gitlab.example.com/group/repo.git":        "gitlab.example.com",
		"": "",
	}
	for in, want := range cases {
		Expect(HostFromRepositoryURL(in)).To(Equal(want), "for %q", in)
	}
}

// A port belongs in the URL but not in the ssh-keyscan argument, which would
// otherwise produce a command that does not work.
func TestHostFromRepositoryURL_DropsThePort(t *testing.T) {
	RegisterTestingT(t)

	host := HostFromRepositoryURL("ssh://git@gitlab.example.com:2222/group/repo.git")
	Expect(host).To(Equal("gitlab.example.com"))
	Expect(host).ToNot(ContainSubstring("2222"))
}

// --- the inputs themselves ---

// An agent reads an input's LABEL as the parameter description and never sees
// the placeholder — and it was an agent that reported this gap. If the guidance
// migrates into a placeholder, the next agent is stuck in exactly the same way.
func TestHostKeyInputs_CarryTheirGuidanceInTheLabel(t *testing.T) {
	RegisterTestingT(t)

	byName := map[string]core.Connection{}
	for _, c := range HostKeyInputs {
		byName[c.Name] = c
	}

	hostKey := byName["host_key"]
	Expect(hostKey.Label).To(ContainSubstring("ssh-keyscan"))
	Expect(hostKey.Label).To(ContainSubstring("SHA256"))
	Expect(hostKey.Visible).ToNot(BeNil(), "only meaningful for SSH auth")
	Expect(hostKey.Visible.Values).To(Equal([]string{"ssh"}))

	skip := byName["skip_host_key_verification"]
	Expect(skip.Type).To(Equal(core.ConnectionTypeBoolean))
	Expect(strings.ToUpper(skip.Label)).To(ContainSubstring("INSECURE"), "the risk must be stated where it is read")
	Expect(skip.Required).To(BeFalse(), "skipping must never be forced on anyone")
}
