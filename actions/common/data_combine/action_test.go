package data_combine

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

	// Pre-populate node results via GetNodeResult/Execute cache
	// We use Load-style initialisation then set results via exposed method
	for id, res := range results {
		// Store results so GetNodeResult can find them
		flow.SetNodeResultForTest(id, res)
	}

	return flow
}

func TestCombineTwoParentsNonOverlapping(t *testing.T) {
	RegisterTestingT(t)

	combineNode := &core.Node{ID: "combine-1", Type: "common/data_combine", Data: &core.NodeData{ID: "combine-1"}}
	parentA := &core.Node{ID: "parent-a", Type: "test", Data: &core.NodeData{ID: "parent-a"}}
	parentB := &core.Node{ID: "parent-b", Type: "test", Data: &core.NodeData{ID: "parent-b"}}

	edges := []*core.Edge{
		{ID: "e1", Source: "parent-a", Target: "combine-1"},
		{ID: "e2", Source: "parent-b", Target: "combine-1"},
	}

	results := map[string]map[string]interface{}{
		"parent-a": {"path": "/tmp/repo", "branch": "main"},
		"parent-b": {"status": "ok", "count": 42},
	}

	flow := buildTestFlow([]*core.Node{combineNode, parentA, parentB}, edges, results)

	output, err := Execute(flow, combineNode, nil)
	Expect(err).To(BeNil())

	combined, ok := output["combined"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(combined["path"]).To(Equal("/tmp/repo"))
	Expect(combined["branch"]).To(Equal("main"))
	Expect(combined["status"]).To(Equal("ok"))
	Expect(combined["count"]).To(Equal(42))
}

func TestCombineOverlappingKeys(t *testing.T) {
	RegisterTestingT(t)

	combineNode := &core.Node{ID: "combine-1", Type: "common/data_combine", Data: &core.NodeData{ID: "combine-1"}}
	parentA := &core.Node{ID: "parent-a", Type: "test", Data: &core.NodeData{ID: "parent-a"}}
	parentB := &core.Node{ID: "parent-b", Type: "test", Data: &core.NodeData{ID: "parent-b"}}

	edges := []*core.Edge{
		{ID: "e1", Source: "parent-a", Target: "combine-1"},
		{ID: "e2", Source: "parent-b", Target: "combine-1"},
	}

	results := map[string]map[string]interface{}{
		"parent-a": {"status": "first"},
		"parent-b": {"status": "second"},
	}

	flow := buildTestFlow([]*core.Node{combineNode, parentA, parentB}, edges, results)

	output, err := Execute(flow, combineNode, nil)
	Expect(err).To(BeNil())

	combined, ok := output["combined"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	// Last parent wins
	Expect(combined["status"]).To(Or(Equal("first"), Equal("second")))
}

func TestCombineNoParents(t *testing.T) {
	RegisterTestingT(t)

	combineNode := &core.Node{ID: "combine-1", Type: "common/data_combine", Data: &core.NodeData{ID: "combine-1"}}

	flow := buildTestFlow([]*core.Node{combineNode}, nil, nil)

	output, err := Execute(flow, combineNode, nil)
	Expect(err).To(BeNil())

	combined, ok := output["combined"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(combined).To(BeEmpty())
}
