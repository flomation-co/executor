package git_tag

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

func Test_LightweightTag(t *testing.T) {
	RegisterTestingT(t)
	dir := initTestRepoWithCommit(t)
	chdir(t, dir)

	result, err := Execute(nil, nil, []*core.Connection{
		conn("tag_name", "v1.0.0"),
		textConn("message", ""),
	})
	Expect(err).To(BeNil())
	Expect(result["tag_name"]).To(Equal("v1.0.0"))

	r, _ := git.PlainOpen(".")
	ref, err := r.Tag("v1.0.0")
	Expect(err).To(BeNil())
	Expect(ref).ToNot(BeNil())
}

func Test_AnnotatedTag(t *testing.T) {
	RegisterTestingT(t)
	dir := initTestRepoWithCommit(t)
	chdir(t, dir)

	result, err := Execute(nil, nil, []*core.Connection{
		conn("tag_name", "v2.0.0"),
		textConn("message", "Release version 2.0.0"),
	})
	Expect(err).To(BeNil())
	Expect(result["tag_name"]).To(Equal("v2.0.0"))

	r, _ := git.PlainOpen(".")
	ref, err := r.Tag("v2.0.0")
	Expect(err).To(BeNil())
	Expect(ref).ToNot(BeNil())
}

func Test_Tag_InvalidRepo(t *testing.T) {
	RegisterTestingT(t)
	dir := t.TempDir()
	chdir(t, dir)

	_, err := Execute(nil, nil, []*core.Connection{
		conn("tag_name", "v1.0.0"),
		textConn("message", ""),
	})
	Expect(err).ToNot(BeNil())
}
