package file_write

import (
	"os"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// Rewritten when Write File was confined to the flow's workspace (AUDIT.md,
// 2026-09-14). The previous tests wrote to an absolute temp path — the same
// capability that let the action write anywhere the service user could,
// including the directories the platform runs from.
//
// These chdir, so they must not run in parallel.

func strConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// enter moves into a throwaway workspace for the duration of a test.
//
// Every test here MUST use it. An absolute path is neutralised into a
// workspace-relative one, and the workspace is whatever the process working
// directory happens to be — so a test that forgets to chdir writes its
// artefacts into the source tree. That is exactly how actions/file/write/etc
// and actions/file/write/var/folders/... briefly reached main.
func enter(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(ws); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	return ws
}

func Test_WriteFile(t *testing.T) {
	RegisterTestingT(t)
	ws := enter(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", "out.txt"),
		strConn("content", "hello world"),
	})
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(true))
	Expect(result["bytes_written"]).To(Equal(11))

	written, err := os.ReadFile(filepath.Join(ws, "out.txt"))
	Expect(err).To(BeNil())
	Expect(string(written)).To(Equal("hello world"))
}

func Test_WriteFile_Append(t *testing.T) {
	RegisterTestingT(t)
	ws := enter(t)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", "out.txt"), strConn("content", "first"),
	})
	Expect(err).To(BeNil())

	_, err = Execute(nil, nil, []*core.Connection{
		strConn("file_path", "out.txt"), strConn("content", " second"), strConn("append", "true"),
	})
	Expect(err).To(BeNil())

	written, err := os.ReadFile(filepath.Join(ws, "out.txt"))
	Expect(err).To(BeNil())
	Expect(string(written)).To(Equal("first second"))
}

// Writing a nested path works without a separate Create Directory step.
func Test_WriteFile_CreatesParentFolders(t *testing.T) {
	RegisterTestingT(t)
	ws := enter(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", "reports/2026/summary.txt"), strConn("content", "x"),
	})
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(true))

	_, err = os.Stat(filepath.Join(ws, "reports", "2026", "summary.txt"))
	Expect(err).To(BeNil())
}

// The case that motivated the change: writing outside the workspace is how a
// read-only leak becomes a takeover.
func Test_WriteFile_CannotEscapeTheWorkspace(t *testing.T) {
	RegisterTestingT(t)
	base := t.TempDir()
	target := filepath.Join(base, "config.json")
	Expect(os.WriteFile(target, []byte("ORIGINAL"), 0o644)).To(BeNil())

	enter(t)

	// An absolute path is neutralised into a workspace-relative one, so the
	// write SUCCEEDS but lands inside the workspace; a traversal reaches
	// os.Root and is refused. Either way the file outside is untouched, which
	// is the property under test — asserting "every call must error" would be
	// testing the mechanism rather than the guarantee.
	for _, attempt := range []string{target, "../config.json"} {
		_, err := Execute(nil, nil, []*core.Connection{
			strConn("file_path", attempt), strConn("content", "OVERWRITTEN"),
		})
		Expect(err).To(BeNil())

		after, readErr := os.ReadFile(target)
		Expect(readErr).To(BeNil(), "the file outside was removed via %q", attempt)
		Expect(string(after)).To(Equal("ORIGINAL"), "a file outside the workspace was modified via %q", attempt)
	}

	// Nothing new appeared beside it either.
	entries, err := os.ReadDir(base)
	Expect(err).To(BeNil())
	Expect(entries).To(HaveLen(1), "a file was created outside the workspace")
}

func Test_WriteFile_RequiresPathAndContent(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	result, err := Execute(nil, nil, []*core.Connection{strConn("content", "x")})
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(false))
	Expect(result["error"]).To(ContainSubstring("required"))

	result, err = Execute(nil, nil, []*core.Connection{strConn("file_path", "a.txt")})
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(false))
	Expect(result["error"]).To(ContainSubstring("content is required"))
}
