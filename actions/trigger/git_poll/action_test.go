package git_poll

import (
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
	Expect(len(Inputs)).To(Equal(4))
	Expect(len(Outputs)).To(Equal(4))
	Expect(Inputs[0].Required).To(BeTrue())
}
