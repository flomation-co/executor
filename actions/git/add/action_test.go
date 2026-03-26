package git_add

import (
	"os"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	"github.com/go-git/go-git/v6"
	. "github.com/onsi/gomega"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

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

func Test_AddSpecificFile(t *testing.T) {
	RegisterTestingT(t)
	dir := initTestRepoWithCommit(t)
	chdir(t, dir)

	Expect(os.WriteFile("test.txt", []byte("hello"), 0644)).To(Succeed())
	_, err := Execute(nil, nil, []*core.Connection{conn("path", "test.txt")})
	Expect(err).To(BeNil())
}

func Test_AddAll(t *testing.T) {
	RegisterTestingT(t)
	dir := initTestRepoWithCommit(t)
	chdir(t, dir)

	Expect(os.WriteFile("a.txt", []byte("a"), 0644)).To(Succeed())
	Expect(os.WriteFile("b.txt", []byte("b"), 0644)).To(Succeed())
	_, err := Execute(nil, nil, []*core.Connection{conn("path", ".")})
	Expect(err).To(BeNil())
}

func Test_AddAll_EmptyPath(t *testing.T) {
	RegisterTestingT(t)
	dir := initTestRepoWithCommit(t)
	chdir(t, dir)

	Expect(os.WriteFile("c.txt", []byte("c"), 0644)).To(Succeed())
	_, err := Execute(nil, nil, []*core.Connection{conn("path", "")})
	Expect(err).To(BeNil())
}

func Test_Add_InvalidRepo(t *testing.T) {
	RegisterTestingT(t)
	dir := t.TempDir()
	chdir(t, dir)

	_, err := Execute(nil, nil, []*core.Connection{conn("path", "test.txt")})
	Expect(err).ToNot(BeNil())
}
