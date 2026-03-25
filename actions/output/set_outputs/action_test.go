package output_set_outputs

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func buildTestFlow(nodes []*core.Node, edges []*core.Edge, results map[string]map[string]interface{}) *core.Flow {
	flow := &core.Flow{
		Nodes: nodes,
		Edges: edges,
	}

	for id, res := range results {
		flow.SetNodeResultForTest(id, res)
	}

	return flow
}

func TestSetOutputsTwoParents(t *testing.T) {
	RegisterTestingT(t)

	setNode := &core.Node{ID: "set-1", Type: "output/set_outputs", Data: &core.NodeData{ID: "set-1"}}
	parentA := &core.Node{ID: "parent-a", Type: "test", Data: &core.NodeData{ID: "parent-a"}}
	parentB := &core.Node{ID: "parent-b", Type: "test", Data: &core.NodeData{ID: "parent-b"}}

	edges := []*core.Edge{
		{ID: "e1", Source: "parent-a", Target: "set-1"},
		{ID: "e2", Source: "parent-b", Target: "set-1"},
	}

	results := map[string]map[string]interface{}{
		"parent-a": {"path": "/tmp/repo", "branch": "main"},
		"parent-b": {"status": "ok", "count": 42},
	}

	flow := buildTestFlow([]*core.Node{setNode, parentA, parentB}, edges, results)

	output, err := Execute(flow, setNode, nil)
	Expect(err).To(BeNil())
	Expect(output["count"]).To(Equal(int64(4)))

	// Verify flow outputs were set
	flowOutputs := flow.GetOutputs()
	Expect(flowOutputs["path"]).To(Equal("/tmp/repo"))
	Expect(flowOutputs["branch"]).To(Equal("main"))
	Expect(flowOutputs["status"]).To(Equal("ok"))
	Expect(flowOutputs["count"]).To(Equal(42))
}

func TestSetOutputsNoParents(t *testing.T) {
	RegisterTestingT(t)

	setNode := &core.Node{ID: "set-1", Type: "output/set_outputs", Data: &core.NodeData{ID: "set-1"}}

	flow := buildTestFlow([]*core.Node{setNode}, nil, nil)

	output, err := Execute(flow, setNode, nil)
	Expect(err).To(BeNil())
	Expect(output["count"]).To(Equal(int64(0)))
}

func TestSetOutputsSingleParent(t *testing.T) {
	RegisterTestingT(t)

	setNode := &core.Node{ID: "set-1", Type: "output/set_outputs", Data: &core.NodeData{ID: "set-1"}}
	parent := &core.Node{ID: "parent-1", Type: "test", Data: &core.NodeData{ID: "parent-1"}}

	edges := []*core.Edge{
		{ID: "e1", Source: "parent-1", Target: "set-1"},
	}

	results := map[string]map[string]interface{}{
		"parent-1": {"message": "hello world"},
	}

	flow := buildTestFlow([]*core.Node{setNode, parent}, edges, results)

	output, err := Execute(flow, setNode, nil)
	Expect(err).To(BeNil())
	Expect(output["count"]).To(Equal(int64(1)))

	flowOutputs := flow.GetOutputs()
	Expect(flowOutputs["message"]).To(Equal("hello world"))
}
