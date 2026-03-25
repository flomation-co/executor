package conditional_for

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestForBasic(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "f-1", Data: &core.NodeData{ID: "f-1"}}

	inputs := []*core.Connection{
		{Name: "count", Type: core.ConnectionTypeInteger, Value: "5"},
	}

	output, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(output["result"]).To(BeTrue())
	Expect(output["iterations"]).To(Equal(int64(5)))
	Expect(output["max_iterations"]).To(Equal(int64(5)))
	Expect(output["current_index"]).To(Equal(int64(0)))
}

func TestForWithStart(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "f-1", Data: &core.NodeData{ID: "f-1"}}

	inputs := []*core.Connection{
		{Name: "count", Type: core.ConnectionTypeInteger, Value: "3"},
		{Name: "start", Type: core.ConnectionTypeInteger, Value: "10"},
	}

	output, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(output["current_index"]).To(Equal(int64(10)))
}

func TestForZero(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "f-1", Data: &core.NodeData{ID: "f-1"}}

	inputs := []*core.Connection{
		{Name: "count", Type: core.ConnectionTypeInteger, Value: "0"},
	}

	output, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(output["result"]).To(BeFalse())
}

func TestForMissingCount(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "f-1", Data: &core.NodeData{ID: "f-1"}}

	inputs := []*core.Connection{}

	_, err := Execute(flow, node, inputs)
	Expect(err).ToNot(BeNil())
}
