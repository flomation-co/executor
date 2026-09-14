package git_poll

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func Test_Execute_ReturnsInputsAsOutputs(t *testing.T) {
	t.Parallel()
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "node-1", Type: "trigger/git_poll"}

	inputs := []*core.Connection{
		{Name: "branch", Type: core.ConnectionTypeString, Value: "main"},
		{Name: "commit_hash", Type: core.ConnectionTypeString, Value: "abc123def"},
		{Name: "commit_message", Type: core.ConnectionTypeString, Value: "fix: resolve issue"},
		{Name: "repository_url", Type: core.ConnectionTypeString, Value: "git@github.com:org/repo.git"},
	}

	result, err := Execute(flow, node, inputs)

	Expect(err).To(BeNil())
	Expect(result).To(HaveKeyWithValue("branch", "main"))
	Expect(result).To(HaveKeyWithValue("commit_hash", "abc123def"))
	Expect(result).To(HaveKeyWithValue("commit_message", "fix: resolve issue"))
	Expect(result).To(HaveKeyWithValue("repository_url", "git@github.com:org/repo.git"))
}

func Test_Execute_SkipsNilValues(t *testing.T) {
	t.Parallel()
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "node-1", Type: "trigger/git_poll"}

	inputs := []*core.Connection{
		{Name: "branch", Type: core.ConnectionTypeString, Value: "main"},
		{Name: "commit_hash", Type: core.ConnectionTypeString, Value: nil},
	}

	result, err := Execute(flow, node, inputs)

	Expect(err).To(BeNil())
	Expect(result).To(HaveKeyWithValue("branch", "main"))
	Expect(result).NotTo(HaveKey("commit_hash"))
}

func Test_Metadata(t *testing.T) {
	t.Parallel()
	RegisterTestingT(t)

	Expect(Type).To(Equal(core.ActionTypeTrigger))
	Expect(Name).To(Equal("Git Poll Trigger"))
	Expect(len(Inputs)).To(Equal(6))
	Expect(len(Outputs)).To(Equal(4))
	Expect(Inputs[0].Required).To(BeTrue())
}

// The Launch poller reads these two by name out of the trigger's Data blob, so
// a rename here silently stops host key verification being configurable and the
// poll goes back to failing against the service's empty known_hosts.
func Test_HostKeyInputsExistAndAreOptional(t *testing.T) {
	t.Parallel()
	RegisterTestingT(t)

	byName := map[string]core.Connection{}
	for _, i := range Inputs {
		byName[i.Name] = i
	}

	hostKey, ok := byName["host_key"]
	Expect(ok).To(BeTrue(), "launch/internal/git/poll reads this key")
	Expect(hostKey.Required).To(BeFalse(), "a trusted known_hosts entry is a valid alternative")
	// An agent reads the label as the parameter description and never sees the
	// placeholder, and it was an agent that reported this gap.
	Expect(hostKey.Label).To(ContainSubstring("ssh-keyscan"))

	skip, ok := byName["skip_host_key_verification"]
	Expect(ok).To(BeTrue(), "launch/internal/git/poll reads this key")
	Expect(skip.Type).To(Equal(core.ConnectionTypeBoolean))
	Expect(skip.Required).To(BeFalse(), "skipping must never be forced on anyone")
	Expect(strings.ToUpper(skip.Label)).To(ContainSubstring("INSECURE"))
}
