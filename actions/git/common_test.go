package git_common

import (
	"os"
	"testing"

	"github.com/go-git/go-git/v6"
	. "github.com/onsi/gomega"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
	return dir
}

func Test_GetRepository(t *testing.T) {
	RegisterTestingT(t)

	dir := initTestRepo(t)

	r, err := GetRepository(dir)
	Expect(err).To(BeNil())
	Expect(r).ToNot(BeNil())
}

func Test_GetRepository_InvalidPath(t *testing.T) {
	RegisterTestingT(t)

	_, err := GetRepository("/nonexistent/path")
	Expect(err).ToNot(BeNil())
}

func Test_GetRepositoryAndWorktree(t *testing.T) {
	RegisterTestingT(t)

	dir := initTestRepo(t)

	r, w, err := GetRepositoryAndWorktree(dir)
	Expect(err).To(BeNil())
	Expect(r).ToNot(BeNil())
	Expect(w).ToNot(BeNil())
}

func Test_GetRepositoryAndWorktree_InvalidPath(t *testing.T) {
	RegisterTestingT(t)

	_, _, err := GetRepositoryAndWorktree("/nonexistent/path")
	Expect(err).ToNot(BeNil())
}

func Test_GetWorktree(t *testing.T) {
	RegisterTestingT(t)

	dir := initTestRepo(t)

	w, err := GetWorktree(dir)
	Expect(err).To(BeNil())
	Expect(w).ToNot(BeNil())
}

func Test_GetSSHAuth(t *testing.T) {
	RegisterTestingT(t)

	keyPath := os.Getenv("FLOMATION_EXECUTOR_TEST_SSH_KEY_PATH")
	if keyPath == "" {
		t.Skip("FLOMATION_EXECUTOR_TEST_SSH_KEY_PATH not set")
	}

	b, err := os.ReadFile(keyPath)
	Expect(err).To(BeNil())

	auth, err := GetSSHAuth(string(b))
	Expect(err).To(BeNil())
	Expect(auth).ToNot(BeNil())
}

func Test_GetSSHAuth_InvalidKey(t *testing.T) {
	RegisterTestingT(t)

	_, err := GetSSHAuth("not-a-valid-key")
	Expect(err).ToNot(BeNil())
}
