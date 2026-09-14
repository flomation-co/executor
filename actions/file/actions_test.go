package file_test

import (
	"os"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	file_copy "flomation.app/automate/executor/actions/file/copy"
	file_create_directory "flomation.app/automate/executor/actions/file/create_directory"
	file_delete "flomation.app/automate/executor/actions/file/delete"
	file_info "flomation.app/automate/executor/actions/file/info"
	file_list "flomation.app/automate/executor/actions/file/list"
	file_move "flomation.app/automate/executor/actions/file/move"
	file_read "flomation.app/automate/executor/actions/file/read"
	file_write "flomation.app/automate/executor/actions/file/write"
	. "github.com/onsi/gomega"
)

// End-to-end through Execute, which is what a flow and an agent actually call.
// These chdir, so they must not run in parallel.

func ins(pairs ...[2]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, &core.Connection{Name: p[0], Type: core.ConnectionTypeString, Value: p[1]})
	}
	return out
}

// enter builds a workspace with a secret outside it and chdirs in.
func enter(t *testing.T) (ws, secret string) {
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
	secret = filepath.Join(outside, "config.json")
	if err := os.WriteFile(secret, []byte("TOP SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink(secret, filepath.Join(ws, "link-to-secret"))

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

// The whole point: an agent asking for the platform's config gets a refusal it
// can understand, not the file.
func TestReadFile_RefusesToLeaveTheWorkspace(t *testing.T) {
	RegisterTestingT(t)
	_, secret := enter(t)

	for _, attempt := range []string{secret, "/etc/passwd", "../outside/config.json", "link-to-secret"} {
		out, err := file_read.Execute(nil, nil, ins([2]string{"file_path", attempt}))
		Expect(err).To(BeNil(), "an refusal is a result, not a node failure")
		Expect(out["success"]).To(Equal(false), "for %q", attempt)
		Expect(out["content"]).To(BeNil(), "for %q", attempt)

		text, _ := out["tool_result"].(string)
		Expect(text).ToNot(ContainSubstring("TOP SECRET"), "the secret leaked for %q", attempt)
	}
}

func TestWriteThenRead_RoundTripsInsideTheWorkspace(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	// A nested path works without a separate Create Directory step.
	out, err := file_write.Execute(nil, nil, ins(
		[2]string{"file_path", "reports/2026/summary.txt"},
		[2]string{"content", "hello world"},
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["bytes_written"]).To(Equal(11))

	out, err = file_read.Execute(nil, nil, ins([2]string{"file_path", "reports/2026/summary.txt"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["content"]).To(Equal("hello world"))
	// The contents must be IN tool_result, or an agent has to call twice.
	Expect(out["tool_result"]).To(ContainSubstring("hello world"))
}

func TestWriteFile_AppendMode(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	_, err := file_write.Execute(nil, nil, ins([2]string{"file_path", "log.txt"}, [2]string{"content", "one\n"}))
	Expect(err).To(BeNil())
	_, err = file_write.Execute(nil, nil, ins(
		[2]string{"file_path", "log.txt"}, [2]string{"content", "two\n"}, [2]string{"append", "true"}))
	Expect(err).To(BeNil())

	out, _ := file_read.Execute(nil, nil, ins([2]string{"file_path", "log.txt"}))
	Expect(out["content"]).To(Equal("one\ntwo\n"))
}

func TestWriteFile_RefusesToLeaveTheWorkspace(t *testing.T) {
	RegisterTestingT(t)
	_, secret := enter(t)

	original, err := os.ReadFile(secret)
	Expect(err).To(BeNil())

	for _, attempt := range []string{secret, "../outside/config.json", "link-to-secret"} {
		_, err := file_write.Execute(nil, nil, ins(
			[2]string{"file_path", attempt}, [2]string{"content", "OVERWRITTEN"}))
		Expect(err).To(BeNil())
	}

	after, err := os.ReadFile(secret)
	Expect(err).To(BeNil())
	Expect(after).To(Equal(original), "a file outside the workspace was modified")
}

func TestCreateDirectory_IsIdempotent(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	out, err := file_create_directory.Execute(nil, nil, ins([2]string{"directory", "reports/2026"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["created"]).To(Equal(true))

	// A flow that makes its output folder every run must not fail the second time.
	out, err = file_create_directory.Execute(nil, nil, ins([2]string{"directory", "reports/2026"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["created"]).To(Equal(false), "reported honestly rather than claiming a fresh create")
}

func TestList_FiltersSortsAndRecurses(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	for _, f := range []string{"b.csv", "a.txt", "sub/c.csv"} {
		_, err := file_write.Execute(nil, nil, ins([2]string{"file_path", f}, [2]string{"content", "x"}))
		Expect(err).To(BeNil())
	}

	// Shallow: the sub-folder's contents must not appear.
	out, err := file_list.Execute(nil, nil, ins())
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	paths, _ := out["paths"].([]string)
	Expect(paths).To(ContainElement("a.txt"))
	Expect(paths).ToNot(ContainElement("sub/c.csv"))

	// Pattern matches on the base name, which is what "*.csv" means to a person.
	out, _ = file_list.Execute(nil, nil, ins([2]string{"pattern", "*.csv"}))
	paths, _ = out["paths"].([]string)
	Expect(paths).To(Equal([]string{"b.csv"}))

	// Recursive reaches into sub-folders.
	out, _ = file_list.Execute(nil, nil, ins([2]string{"pattern", "*.csv"}, [2]string{"recursive", "true"}))
	paths, _ = out["paths"].([]string)
	Expect(paths).To(ContainElement("sub/c.csv"))
	Expect(paths).To(ContainElement("b.csv"))

	// The listing goes IN tool_result so an agent can act on one call.
	Expect(out["tool_result"]).To(ContainSubstring("b.csv"))
}

func TestList_RejectsABadPatternRatherThanReturningNothing(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	// An unmatched [ is a malformed pattern; silently returning zero results
	// reads as "the folder is empty".
	out, err := file_list.Execute(nil, nil, ins([2]string{"pattern", "[bad"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("not a valid pattern"))
}

func TestList_CannotEnumerateOutsideTheWorkspace(t *testing.T) {
	RegisterTestingT(t)
	_, secret := enter(t)

	for _, attempt := range []string{filepath.Dir(secret), "../outside", "link-to-secret"} {
		out, err := file_list.Execute(nil, nil, ins([2]string{"directory", attempt}))
		Expect(err).To(BeNil())
		Expect(out["success"]).To(Equal(false), "for %q", attempt)
		text, _ := out["tool_result"].(string)
		Expect(text).ToNot(ContainSubstring("config.json"), "the folder outside was enumerated via %q", attempt)
	}
}

// Absence is an answer here, not a failure — the action exists to ask.
func TestInfo_ReportsAbsenceAsSuccess(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	out, err := file_info.Execute(nil, nil, ins([2]string{"path", "nothing-here.txt"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["exists"]).To(Equal(false))

	_, err = file_write.Execute(nil, nil, ins([2]string{"file_path", "there.txt"}, [2]string{"content", "abc"}))
	Expect(err).To(BeNil())

	out, err = file_info.Execute(nil, nil, ins([2]string{"path", "there.txt"}))
	Expect(err).To(BeNil())
	Expect(out["exists"]).To(Equal(true))
	Expect(out["is_directory"]).To(Equal(false))
	Expect(out["size"]).To(Equal(3))
}

// Deleting a tree because a path happened to be a directory is the mistake an
// agent makes once and cannot undo.
func TestDelete_NeedsRecursiveForANonEmptyFolder(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	_, err := file_write.Execute(nil, nil, ins([2]string{"file_path", "tree/keep.txt"}, [2]string{"content", "x"}))
	Expect(err).To(BeNil())

	out, err := file_delete.Execute(nil, nil, ins([2]string{"path", "tree"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("not empty"))

	// The file is still there.
	info, _ := file_info.Execute(nil, nil, ins([2]string{"path", "tree/keep.txt"}))
	Expect(info["exists"]).To(Equal(true))

	out, err = file_delete.Execute(nil, nil, ins([2]string{"path", "tree"}, [2]string{"recursive", "true"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["was_directory"]).To(Equal(true))

	info, _ = file_info.Execute(nil, nil, ins([2]string{"path", "tree/keep.txt"}))
	Expect(info["exists"]).To(Equal(false))
}

func TestDelete_RefusesTheWorkspaceItself(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	for _, attempt := range []string{"", ".", "/"} {
		out, err := file_delete.Execute(nil, nil, ins([2]string{"path", attempt}, [2]string{"recursive", "true"}))
		Expect(err).To(BeNil())
		Expect(out["success"]).To(Equal(false), "for %q", attempt)
	}

	// The workspace survived.
	out, _ := file_list.Execute(nil, nil, ins())
	Expect(out["success"]).To(Equal(true))
}

func TestDelete_IgnoreMissing(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	out, err := file_delete.Execute(nil, nil, ins([2]string{"path", "gone.txt"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))

	out, err = file_delete.Execute(nil, nil, ins([2]string{"path", "gone.txt"}, [2]string{"ignore_missing", "true"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["deleted"]).To(Equal(false))
}

func TestDelete_CannotReachOutsideTheWorkspace(t *testing.T) {
	RegisterTestingT(t)
	_, secret := enter(t)

	for _, attempt := range []string{secret, "../outside/config.json", "../outside", "link-to-secret"} {
		_, err := file_delete.Execute(nil, nil, ins(
			[2]string{"path", attempt}, [2]string{"recursive", "true"}, [2]string{"ignore_missing", "true"}))
		Expect(err).To(BeNil())

		_, statErr := os.Stat(secret)
		Expect(statErr).To(BeNil(), "the secret was deleted via %q", attempt)
	}
}

// Rename clobbers silently, which is data loss an agent would never report.
func TestMove_WillNotOverwriteUnlessTold(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	_, err := file_write.Execute(nil, nil, ins([2]string{"file_path", "a.txt"}, [2]string{"content", "A"}))
	Expect(err).To(BeNil())
	_, err = file_write.Execute(nil, nil, ins([2]string{"file_path", "b.txt"}, [2]string{"content", "B"}))
	Expect(err).To(BeNil())

	out, err := file_move.Execute(nil, nil, ins([2]string{"source", "a.txt"}, [2]string{"destination", "b.txt"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("already exists"))

	read, _ := file_read.Execute(nil, nil, ins([2]string{"file_path", "b.txt"}))
	Expect(read["content"]).To(Equal("B"), "the destination must be untouched")

	out, err = file_move.Execute(nil, nil, ins(
		[2]string{"source", "a.txt"}, [2]string{"destination", "b.txt"}, [2]string{"overwrite", "true"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))

	read, _ = file_read.Execute(nil, nil, ins([2]string{"file_path", "b.txt"}))
	Expect(read["content"]).To(Equal("A"))
}

func TestMove_CreatesTheDestinationFolder(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	_, err := file_write.Execute(nil, nil, ins([2]string{"file_path", "in.txt"}, [2]string{"content", "x"}))
	Expect(err).To(BeNil())

	out, err := file_move.Execute(nil, nil, ins(
		[2]string{"source", "in.txt"}, [2]string{"destination", "processed/2026/in.txt"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))

	info, _ := file_info.Execute(nil, nil, ins([2]string{"path", "processed/2026/in.txt"}))
	Expect(info["exists"]).To(Equal(true))
}

func TestMove_CannotMoveAnythingOutOfTheWorkspace(t *testing.T) {
	RegisterTestingT(t)
	_, secret := enter(t)

	_, err := file_write.Execute(nil, nil, ins([2]string{"file_path", "payload.txt"}, [2]string{"content", "data"}))
	Expect(err).To(BeNil())

	original, err := os.ReadFile(secret)
	Expect(err).To(BeNil())

	for _, attempt := range []string{"../outside/stolen.txt", secret} {
		_, err := file_move.Execute(nil, nil, ins(
			[2]string{"source", "payload.txt"}, [2]string{"destination", attempt}, [2]string{"overwrite", "true"}))
		Expect(err).To(BeNil())
		_, _ = file_write.Execute(nil, nil, ins([2]string{"file_path", "payload.txt"}, [2]string{"content", "data"}))

		after, readErr := os.ReadFile(secret)
		Expect(readErr).To(BeNil(), "the secret was moved away via %q", attempt)
		Expect(after).To(Equal(original), "the secret was overwritten via %q", attempt)
	}

	entries, err := os.ReadDir(filepath.Dir(secret))
	Expect(err).To(BeNil())
	Expect(entries).To(HaveLen(1), "a file was placed outside the workspace")
}

func TestCopy_DuplicatesAndRefusesFolders(t *testing.T) {
	RegisterTestingT(t)
	enter(t)

	_, err := file_write.Execute(nil, nil, ins([2]string{"file_path", "src.txt"}, [2]string{"content", "payload"}))
	Expect(err).To(BeNil())

	out, err := file_copy.Execute(nil, nil, ins(
		[2]string{"source", "src.txt"}, [2]string{"destination", "archive/src.txt"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["bytes_copied"]).To(Equal(7))

	// Both still exist — a copy is not a move.
	for _, p := range []string{"src.txt", "archive/src.txt"} {
		info, _ := file_info.Execute(nil, nil, ins([2]string{"path", p}))
		Expect(info["exists"]).To(Equal(true), "for %q", p)
	}

	_, err = file_create_directory.Execute(nil, nil, ins([2]string{"directory", "somedir"}))
	Expect(err).To(BeNil())
	out, err = file_copy.Execute(nil, nil, ins(
		[2]string{"source", "somedir"}, [2]string{"destination", "elsewhere"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("folder"))
}

func TestCopy_CannotReadFromOrWriteToOutside(t *testing.T) {
	RegisterTestingT(t)
	_, secret := enter(t)

	// Copying the secret IN would be exfiltration by another name.
	out, err := file_copy.Execute(nil, nil, ins(
		[2]string{"source", secret}, [2]string{"destination", "stolen.txt"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))

	info, _ := file_info.Execute(nil, nil, ins([2]string{"path", "stolen.txt"}))
	Expect(info["exists"]).To(Equal(false))

	// And copying OUT must not land.
	_, err = file_write.Execute(nil, nil, ins([2]string{"file_path", "x.txt"}, [2]string{"content", "x"}))
	Expect(err).To(BeNil())
	_, err = file_copy.Execute(nil, nil, ins(
		[2]string{"source", "x.txt"}, [2]string{"destination", "../outside/leaked.txt"},
		[2]string{"overwrite", "true"}))
	Expect(err).To(BeNil())

	entries, err := os.ReadDir(filepath.Dir(secret))
	Expect(err).To(BeNil())
	Expect(entries).To(HaveLen(1), "a file was copied outside the workspace")
}
