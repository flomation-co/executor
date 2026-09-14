package file

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

// The blast radius these actions are required to enforce: everything a flow
// does stays inside its own workspace, so a rogue agent or a bad flow cannot
// reach another tenant's data or damage the runner and executor.
//
// Confinement is os.Root, which resolves every path component against a
// directory handle and refuses anything leaving the root — enforced by the
// kernel, so it survives symlinks and the check-then-open race that defeats a
// string comparison. These tests assert that claim against the vectors that
// matter rather than trusting the documentation.
//
// NOTE: these chdir, because Workspace() reads the process working directory
// exactly as it does in production. They must not run in parallel.

// workspace builds a workspace with a secret outside it, chdirs in, and returns
// the path of the secret so a test can assert it survived.
func workspace(t *testing.T) (ws string, secret string) {
	t.Helper()

	base := t.TempDir()
	ws = filepath.Join(base, "workspace")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(ws, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}

	// Stands in for /opt/flomation/api/config.json — the file that holds the
	// database encryption key, and therefore every tenant's credentials.
	secret = filepath.Join(outside, "config.json")
	if err := os.WriteFile(secret, []byte(`{"database":{"encryption_key":"TOP SECRET"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// The escape a path-prefix check cannot see: links planted inside the
	// workspace. A flow can create these itself with Write File.
	_ = os.Symlink(secret, filepath.Join(ws, "link-to-secret"))
	_ = os.Symlink(outside, filepath.Join(ws, "link-to-outside"))

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(ws); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	return ws, secret
}

// escapeAttempts is the list every action is tried against. Each is a path a
// hostile or confused caller could supply.
func escapeAttempts(secret string) map[string]string {
	return map[string]string{
		"absolute path to the secret": secret,
		"absolute system path":        "/etc/passwd",
		"parent traversal":            "../outside/config.json",
		"deep traversal":              "../../../../../../etc/passwd",
		"traversal after a segment":   "sub/../../outside/config.json",
		"symlink to the secret":       "link-to-secret",
		"through a symlinked folder":  "link-to-outside/config.json",
		"absolute with traversal":     "/../outside/config.json",
	}
}

// Reading must never return the contents of a file outside the workspace.
func TestConfinement_ReadCannotEscape(t *testing.T) {
	RegisterTestingT(t)
	_, secret := workspace(t)

	for label, attempt := range escapeAttempts(secret) {
		root, err := Workspace()
		Expect(err).To(BeNil())

		content, readErr := root.ReadFile(Rel(attempt))
		_ = root.Close()

		Expect(readErr).ToNot(BeNil(), "read must be refused: %s (%s)", label, attempt)
		Expect(string(content)).ToNot(ContainSubstring("TOP SECRET"), "the secret leaked via: %s", label)
	}
}

// Writing must never land outside the workspace — that is the path from "read a
// file" to "own the platform", by replacing a binary or a config.
func TestConfinement_WriteCannotEscape(t *testing.T) {
	RegisterTestingT(t)
	_, secret := workspace(t)

	original, err := os.ReadFile(secret)
	Expect(err).To(BeNil())

	for label, attempt := range escapeAttempts(secret) {
		root, err := Workspace()
		Expect(err).To(BeNil())

		writeErr := root.WriteFile(Rel(attempt), []byte("OVERWRITTEN"), 0o640)
		_ = root.Close()

		Expect(writeErr).ToNot(BeNil(), "write must be refused: %s (%s)", label, attempt)
	}

	after, err := os.ReadFile(secret)
	Expect(err).To(BeNil())
	Expect(after).To(Equal(original), "a file outside the workspace was modified")
}

// Deleting outside the workspace would let one flow destroy another's data, or
// the runner's own files.
func TestConfinement_DeleteCannotEscape(t *testing.T) {
	RegisterTestingT(t)
	_, secret := workspace(t)

	// What matters is the OUTCOME, not that every call errors. An absolute path
	// is neutralised by Rel into a workspace-relative one, so RemoveAll on it is
	// a no-op returning nil — correct, and nothing outside is touched. A
	// traversal form reaches os.Root and is refused outright.
	for label, attempt := range escapeAttempts(secret) {
		root, err := Workspace()
		Expect(err).To(BeNil())

		rel := Rel(attempt)
		_ = root.Remove(rel)
		_ = root.RemoveAll(rel)
		_ = root.Close()

		_, statErr := os.Stat(secret)
		Expect(statErr).To(BeNil(), "the secret outside the workspace was deleted via: %s", label)
	}

	outside := filepath.Dir(secret)
	entries, err := os.ReadDir(outside)
	Expect(err).To(BeNil(), "the folder outside the workspace was removed")
	Expect(entries).ToNot(BeEmpty(), "the folder outside the workspace was emptied")
}

// Creating folders outside would let a flow scatter state across the host.
func TestConfinement_MkdirCannotEscape(t *testing.T) {
	RegisterTestingT(t)
	ws, secret := workspace(t)

	// An absolute path becomes a deep but CONTAINED tree inside the workspace,
	// which is ugly and harmless; a traversal is refused. Either way nothing may
	// appear outside, which is the property under test.
	for _, attempt := range escapeAttempts(secret) {
		root, err := Workspace()
		Expect(err).To(BeNil())
		_ = root.MkdirAll(Rel(attempt)+"/created", 0o750)
		_ = root.Close()
	}

	siblings, err := os.ReadDir(filepath.Dir(ws))
	Expect(err).To(BeNil())
	for _, s := range siblings {
		Expect(s.Name()).To(BeElementOf("workspace", "outside"), "an unexpected entry appeared beside the workspace")
	}
	// And the folder outside gained nothing.
	outsideEntries, err := os.ReadDir(filepath.Dir(secret))
	Expect(err).To(BeNil())
	Expect(outsideEntries).To(HaveLen(1), "something was created outside the workspace")
}

// Renaming out of the workspace would move a flow's data somewhere it can be
// read by another execution.
func TestConfinement_RenameCannotEscape(t *testing.T) {
	RegisterTestingT(t)
	_, secret := workspace(t)

	root, err := Workspace()
	Expect(err).To(BeNil())
	Expect(root.WriteFile("payload.txt", []byte("data"), 0o640)).To(BeNil())
	_ = root.Close()

	original, err := os.ReadFile(secret)
	Expect(err).To(BeNil())

	// Renaming ONTO a symlink name replaces the link itself rather than
	// following it, so the target outside is untouched — which is the property
	// that matters, not whether the call errored.
	for label, attempt := range escapeAttempts(secret) {
		root, err := Workspace()
		Expect(err).To(BeNil())
		_ = root.Rename("payload.txt", Rel(attempt))
		_ = root.WriteFile("payload.txt", []byte("data"), 0o640)
		_ = root.Close()

		after, readErr := os.ReadFile(secret)
		Expect(readErr).To(BeNil(), "the secret was moved away via: %s", label)
		Expect(after).To(Equal(original), "the secret was overwritten via: %s", label)
	}
}

// A listing must not enumerate anything outside, including through a planted
// symlink — the directory names alone are a disclosure.
func TestConfinement_ListCannotEscape(t *testing.T) {
	RegisterTestingT(t)
	_, secret := workspace(t)

	root, err := Workspace()
	Expect(err).To(BeNil())
	defer func() { _ = root.Close() }()

	for label, attempt := range escapeAttempts(secret) {
		f, openErr := root.Open(Rel(attempt))
		if openErr == nil {
			_ = f.Close()
		}
		Expect(openErr).ToNot(BeNil(), "listing must be refused: %s", label)
	}
}

// Legitimate work inside the workspace must still function — a confinement that
// blocks the happy path is useless.
func TestConfinement_AllowsOrdinaryWorkInsideTheWorkspace(t *testing.T) {
	RegisterTestingT(t)
	workspace(t)

	root, err := Workspace()
	Expect(err).To(BeNil())
	defer func() { _ = root.Close() }()

	Expect(root.MkdirAll("reports/2026", 0o750)).To(BeNil())
	Expect(root.WriteFile("reports/2026/summary.txt", []byte("hello"), 0o640)).To(BeNil())

	content, err := root.ReadFile("reports/2026/summary.txt")
	Expect(err).To(BeNil())
	Expect(string(content)).To(Equal("hello"))

	Expect(root.Rename("reports/2026/summary.txt", "reports/final.txt")).To(BeNil())
	Expect(root.Remove("reports/final.txt")).To(BeNil())
	Expect(root.RemoveAll("reports")).To(BeNil())
}

// Rel is a convenience, NOT the boundary. It exists so an agent writing
// "/output/x" gets something sensible rather than an error — but every path it
// returns still goes through os.Root, which is what the tests above prove.
func TestRel_TreatsLeadingSlashAsTheWorkspaceRoot(t *testing.T) {
	RegisterTestingT(t)

	Expect(Rel("/output/report.txt")).To(Equal("output/report.txt"))
	Expect(Rel("output/report.txt")).To(Equal("output/report.txt"))
	Expect(Rel("./output/report.txt")).To(Equal("output/report.txt"))
	Expect(Rel("  reports/x.csv  ")).To(Equal("reports/x.csv"))

	// The workspace itself, however it is spelled.
	for _, raw := range []string{"", ".", "/", "./"} {
		Expect(Rel(raw)).To(Equal("."), "for %q", raw)
	}

	// Traversal is NOT silently stripped — it is passed through for os.Root to
	// refuse. Quietly rewriting it would hide an attempt that should be visible.
	Expect(Rel("../outside/config.json")).To(Equal("../outside/config.json"))
}

func TestRequirePath_RefusesTheWorkspaceRoot(t *testing.T) {
	RegisterTestingT(t)

	_, err := RequirePath("", "file_path")
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("relative to the flow's workspace"))

	_, err = RequirePath("/", "file_path")
	Expect(err).ToNot(BeNil())

	p, err := RequirePath("/reports/x.csv", "file_path")
	Expect(err).To(BeNil())
	Expect(p).To(Equal("reports/x.csv"))
}

// The refusal has to explain the model, not the syscall: "path escapes from
// parent" tells a flow author — or an agent — nothing they can act on.
func TestDescribeError_ExplainsTheWorkspaceRule(t *testing.T) {
	RegisterTestingT(t)
	_, secret := workspace(t)

	root, err := Workspace()
	Expect(err).To(BeNil())
	defer func() { _ = root.Close() }()

	// A traversal reaches os.Root and is refused there; an absolute path is
	// neutralised earlier by Rel and reads as "does not exist", which is also a
	// fair description of <workspace>/etc/passwd.
	traversal := "../" + filepath.Base(filepath.Dir(secret)) + "/config.json"
	_, readErr := root.ReadFile(Rel(traversal))
	Expect(readErr).ToNot(BeNil())

	described := DescribeError(readErr, traversal)
	Expect(described.Error()).To(ContainSubstring("outside the flow's workspace"))
	Expect(described.Error()).To(ContainSubstring("no way to read or write anywhere else"))
	Expect(described.Error()).ToNot(ContainSubstring("escapes from parent"))

	Expect(DescribeError(nil, "x")).To(BeNil())
}
