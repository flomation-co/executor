package data_extract

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

func TestExtractSubsetOfKeys(t *testing.T) {
	RegisterTestingT(t)

	extractNode := &core.Node{ID: "extract-1", Type: "common/data_extract", Data: &core.NodeData{ID: "extract-1"}}
	parentA := &core.Node{ID: "parent-a", Type: "test", Data: &core.NodeData{ID: "parent-a"}}

	edges := []*core.Edge{
		{ID: "e1", Source: "parent-a", Target: "extract-1"},
	}

	results := map[string]map[string]interface{}{
		"parent-a": {"path": "/tmp/repo", "branch": "main", "status": "ok"},
	}

	flow := buildTestFlow([]*core.Node{extractNode, parentA}, edges, results)

	inputs := []*core.Connection{
		{Name: "keys", Type: core.ConnectionTypeText, Value: "path, status"},
	}

	output, err := Execute(flow, extractNode, inputs)
	Expect(err).To(BeNil())

	extracted, ok := output["extracted"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(extracted).To(HaveLen(2))
	Expect(extracted["path"]).To(Equal("/tmp/repo"))
	Expect(extracted["status"]).To(Equal("ok"))
	Expect(extracted).NotTo(HaveKey("branch"))
}

func TestExtractNonExistentKey(t *testing.T) {
	RegisterTestingT(t)

	extractNode := &core.Node{ID: "extract-1", Type: "common/data_extract", Data: &core.NodeData{ID: "extract-1"}}
	parentA := &core.Node{ID: "parent-a", Type: "test", Data: &core.NodeData{ID: "parent-a"}}

	edges := []*core.Edge{
		{ID: "e1", Source: "parent-a", Target: "extract-1"},
	}

	results := map[string]map[string]interface{}{
		"parent-a": {"path": "/tmp/repo"},
	}

	flow := buildTestFlow([]*core.Node{extractNode, parentA}, edges, results)

	inputs := []*core.Connection{
		{Name: "keys", Type: core.ConnectionTypeText, Value: "path, nonexistent"},
	}

	output, err := Execute(flow, extractNode, inputs)
	Expect(err).To(BeNil())

	extracted, ok := output["extracted"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(extracted).To(HaveLen(1))
	Expect(extracted["path"]).To(Equal("/tmp/repo"))
}

func TestExtractEmptyKeys(t *testing.T) {
	RegisterTestingT(t)

	extractNode := &core.Node{ID: "extract-1", Type: "common/data_extract", Data: &core.NodeData{ID: "extract-1"}}

	flow := buildTestFlow([]*core.Node{extractNode}, nil, nil)

	inputs := []*core.Connection{
		{Name: "keys", Type: core.ConnectionTypeText, Value: ""},
	}

	output, err := Execute(flow, extractNode, inputs)
	Expect(err).To(BeNil())

	extracted, ok := output["extracted"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(extracted).To(BeEmpty())
}
