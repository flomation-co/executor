package git_commit

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

func textConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: value}
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

func Test_Commit(t *testing.T) {
	RegisterTestingT(t)

	dir := initTestRepoWithCommit(t)

	// Create and stage a file
	err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("content"), 0644)
	Expect(err).To(BeNil())

	r, _ := git.PlainOpen(dir)
	w, _ := r.Worktree()
	_, err = w.Add("new.txt")
	Expect(err).To(BeNil())

	result, err := Execute(nil, nil, []*core.Connection{
		conn("repository_path", dir),
		textConn("message", "Test commit message"),
		conn("author_name", "Test Author"),
		conn("author_email", "test@example.com"),
	})
	Expect(err).To(BeNil())
	Expect(result["repository_path"]).To(Equal(dir))
	Expect(result["commit_hash"]).ToNot(BeEmpty())
}

func Test_Commit_InvalidRepo(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		conn("repository_path", "/nonexistent/path"),
		textConn("message", "msg"),
		conn("author_name", "name"),
		conn("author_email", "email"),
	})
	Expect(err).ToNot(BeNil())
}
