package file_read

import (
	"os"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// These tests changed shape when Read File was confined to the flow's
// workspace (see AUDIT.md, 2026-09-14). They used to pass an absolute temp
// path, which is exactly the thing that no longer works — and is why the
// action could previously read /opt/flomation/api/config.json.
//
// Two contract changes are asserted here:
//   - paths are relative to the workspace, so the test chdirs into one;
//   - a bad path is a success:false RESULT, not a node failure, so an agent
//     gets a readable explanation instead of the flow dying.
//
// They chdir, so they must not run in parallel.

func strConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

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

func Test_ReadFile(t *testing.T) {
	RegisterTestingT(t)
	ws := enter(t)

	Expect(os.WriteFile(filepath.Join(ws, "test.txt"), []byte("hello world"), 0o644)).To(BeNil())

	result, err := Execute(nil, nil, []*core.Connection{strConn("file_path", "test.txt")})
	Expect(err).To(BeNil())
	Expect(result["content"]).To(Equal("hello world"))
	Expect(result["file_name"]).To(Equal("test.txt"))
	Expect(result["file_size"]).To(Equal(11))
	Expect(result["success"]).To(Equal(true))
}

// A leading slash means the workspace root, not the machine root, so an agent
// writing "/data/x.txt" gets something sensible rather than an error.
func Test_ReadFile_LeadingSlashIsTheWorkspaceRoot(t *testing.T) {
	RegisterTestingT(t)
	ws := enter(t)

	Expect(os.WriteFile(filepath.Join(ws, "test.txt"), []byte("hi"), 0o644)).To(BeNil())

	result, err := Execute(nil, nil, []*core.Connection{strConn("file_path", "/test.txt")})
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(true))
	Expect(result["content"]).To(Equal("hi"))
}

func Test_ReadFile_NotFound(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	result, err := Execute(nil, nil, []*core.Connection{strConn("file_path", "missing.txt")})
	Expect(err).To(BeNil(), "a missing file is a result, not a node failure")
	Expect(result["success"]).To(Equal(false))
	Expect(result["error"]).To(ContainSubstring("does not exist"))
}

func Test_ReadFile_Directory(t *testing.T) {
	RegisterTestingT(t)
	ws := enter(t)

	Expect(os.MkdirAll(filepath.Join(ws, "subdir"), 0o750)).To(BeNil())

	result, err := Execute(nil, nil, []*core.Connection{strConn("file_path", "subdir")})
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(false))
	Expect(result["error"]).To(ContainSubstring("List Files"), "the message should name the action that would work")
}

// The case that motivated the change: an absolute path to a real file outside
// the workspace must not be readable.
func Test_ReadFile_CannotEscapeTheWorkspace(t *testing.T) {
	RegisterTestingT(t)
	base := t.TempDir()
	outside := filepath.Join(base, "secret.txt")
	Expect(os.WriteFile(outside, []byte("TOP SECRET"), 0o644)).To(BeNil())

	enter(t)

	for _, attempt := range []string{outside, "../secret.txt", "/etc/passwd"} {
		result, err := Execute(nil, nil, []*core.Connection{strConn("file_path", attempt)})
		Expect(err).To(BeNil())
		Expect(result["success"]).To(Equal(false), "for %q", attempt)
		Expect(result["content"]).To(BeNil(), "for %q", attempt)

		text, _ := result["tool_result"].(string)
		Expect(text).ToNot(ContainSubstring("TOP SECRET"), "the secret leaked via %q", attempt)
	}
}

func Test_ReadFile_RequiresAPath(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	result, err := Execute(nil, nil, []*core.Connection{strConn("file_path", "")})
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(false))
	Expect(result["error"]).To(ContainSubstring("required"))
}
