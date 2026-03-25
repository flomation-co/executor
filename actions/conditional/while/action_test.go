package conditional_while

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestWhileEqualsTrue(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "w-1", Data: &core.NodeData{ID: "w-1"}}

	inputs := []*core.Connection{
		{Name: "value_a", Type: core.ConnectionTypeString, Value: "hello"},
		{Name: "operator", Type: core.ConnectionTypeString, Value: "equals"},
		{Name: "value_b", Type: core.ConnectionTypeString, Value: "hello"},
	}

	output, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(output["result"]).To(BeTrue())
}

func TestWhileEqualsFalse(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "w-1", Data: &core.NodeData{ID: "w-1"}}

	inputs := []*core.Connection{
		{Name: "value_a", Type: core.ConnectionTypeString, Value: "hello"},
		{Name: "operator", Type: core.ConnectionTypeString, Value: "equals"},
		{Name: "value_b", Type: core.ConnectionTypeString, Value: "world"},
	}

	output, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(output["result"]).To(BeFalse())
}

func TestWhileMaxIterations(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "w-1", Data: &core.NodeData{ID: "w-1"}}

	inputs := []*core.Connection{
		{Name: "value_a", Type: core.ConnectionTypeString, Value: "a"},
		{Name: "operator", Type: core.ConnectionTypeString, Value: "equals"},
		{Name: "value_b", Type: core.ConnectionTypeString, Value: "a"},
		{Name: "max_iterations", Type: core.ConnectionTypeInteger, Value: "50"},
	}

	output, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(output["max_iterations"]).To(Equal(int64(50)))
}

func TestWhileMissingOperator(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "w-1", Data: &core.NodeData{ID: "w-1"}}

	inputs := []*core.Connection{
		{Name: "value_a", Type: core.ConnectionTypeString, Value: "hello"},
	}

	_, err := Execute(flow, node, inputs)
	Expect(err).ToNot(BeNil())
}
