package set_variable

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestSetVariable(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "sv-1", Data: &core.NodeData{ID: "sv-1"}}

	inputs := []*core.Connection{
		{Name: "name", Type: core.ConnectionTypeString, Value: "base_url"},
		{Name: "value", Type: core.ConnectionTypeString, Value: "https://api.example.com"},
	}

	output, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(output["name"]).To(Equal("base_url"))
	Expect(output["value"]).To(Equal("https://api.example.com"))

	// Verify variable was set on the flow
	val, ok := flow.GetVariable("base_url")
	Expect(ok).To(BeTrue())
	Expect(val).To(Equal("https://api.example.com"))
}

func TestSetVariableEmptyName(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "sv-1", Data: &core.NodeData{ID: "sv-1"}}

	inputs := []*core.Connection{
		{Name: "name", Type: core.ConnectionTypeString, Value: ""},
		{Name: "value", Type: core.ConnectionTypeString, Value: "test"},
	}

	_, err := Execute(flow, node, inputs)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("variable name is required"))
}

func TestSetVariableOverwrite(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "sv-1", Data: &core.NodeData{ID: "sv-1"}}

	// Set first time
	inputs1 := []*core.Connection{
		{Name: "name", Type: core.ConnectionTypeString, Value: "counter"},
		{Name: "value", Type: core.ConnectionTypeString, Value: "first"},
	}
	_, err := Execute(flow, node, inputs1)
	Expect(err).To(BeNil())

	// Overwrite
	inputs2 := []*core.Connection{
		{Name: "name", Type: core.ConnectionTypeString, Value: "counter"},
		{Name: "value", Type: core.ConnectionTypeString, Value: "second"},
	}
	_, err = Execute(flow, node, inputs2)
	Expect(err).To(BeNil())

	val, ok := flow.GetVariable("counter")
	Expect(ok).To(BeTrue())
	Expect(val).To(Equal("second"))
}
