package git_clone

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
	. "github.com/onsi/gomega"
)

func Test_SSHKey(t *testing.T) {
	RegisterTestingT(t)

	keyPath := os.Getenv("FLOMATION_EXECUTOR_TEST_SSH_KEY_PATH")
	Expect(keyPath).To(Not(BeEmpty()))

	b, err := os.ReadFile(keyPath)
	Expect(err).To(BeNil())
	Expect(b).To(Not(BeNil()))

	fmt.Printf("%v\n", string(b))

	pk, err := ssh.NewPublicKeys("git", b, "")
	Expect(err).To(BeNil())
	Expect(pk).To(Not(BeNil()))

	fmt.Printf("%v\n", pk.String())
}
