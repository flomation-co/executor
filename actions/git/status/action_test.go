package git_status

import (
	"os"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	"github.com/go-git/go-git/v6"
	. "github.com/onsi/gomega"
)

func initTestRepoWithCommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	r, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}
	if _, err := w.Add("README.md"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if _, err := w.Commit("Initial commit", &git.CommitOptions{}); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	return dir
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

func Test_Status_Clean(t *testing.T) {
	RegisterTestingT(t)

	dir := initTestRepoWithCommit(t)
	chdir(t, dir)

	result, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(result["is_clean"]).To(Equal("true"))
}

func Test_Status_Dirty(t *testing.T) {
	RegisterTestingT(t)

	dir := initTestRepoWithCommit(t)
	chdir(t, dir)

	err := os.WriteFile("untracked.txt", []byte("dirty"), 0644)
	Expect(err).To(BeNil())

	result, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(result["is_clean"]).To(Equal("false"))
	Expect(result["status"]).ToNot(BeEmpty())
}

func Test_Status_InvalidRepo(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	chdir(t, dir)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).ToNot(BeNil())
}
