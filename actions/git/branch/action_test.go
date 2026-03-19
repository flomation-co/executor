package git_branch

import (
	"os"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	"github.com/go-git/go-git/v6"
	. "github.com/onsi/gomega"
)

func conn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

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

func Test_Branch(t *testing.T) {
	RegisterTestingT(t)

	dir := initTestRepoWithCommit(t)

	result, err := Execute(nil, nil, []*core.Connection{
		conn("repository_path", dir),
		conn("branch_name", "feature/test-branch"),
	})
	Expect(err).To(BeNil())
	Expect(result["repository_path"]).To(Equal(dir))

	// Verify we're on the new branch
	r, _ := git.PlainOpen(dir)
	head, _ := r.Head()
	Expect(head.Name().Short()).To(Equal("feature/test-branch"))
}

func Test_Branch_InvalidRepo(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		conn("repository_path", "/nonexistent/path"),
		conn("branch_name", "test"),
	})
	Expect(err).ToNot(BeNil())
}
