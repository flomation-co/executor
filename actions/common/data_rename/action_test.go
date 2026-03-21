package data_rename

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

func TestRenameBasic(t *testing.T) {
	RegisterTestingT(t)

	renameNode := &core.Node{ID: "rename-1", Type: "common/data_rename", Data: &core.NodeData{ID: "rename-1"}}
	parentA := &core.Node{ID: "parent-a", Type: "test", Data: &core.NodeData{ID: "parent-a"}}

	edges := []*core.Edge{
		{ID: "e1", Source: "parent-a", Target: "rename-1"},
	}

	results := map[string]map[string]interface{}{
		"parent-a": {"repository_path": "/tmp/repo", "branch": "main"},
	}

	flow := buildTestFlow([]*core.Node{renameNode, parentA}, edges, results)

	inputs := []*core.Connection{
		{Name: "input_key", Type: core.ConnectionTypeString, Value: "repository_path"},
		{Name: "output_key", Type: core.ConnectionTypeString, Value: "path"},
	}

	output, err := Execute(flow, renameNode, inputs)
	Expect(err).To(BeNil())
	Expect(output["path"]).To(Equal("/tmp/repo"))
	Expect(output).NotTo(HaveKey("repository_path"))
}

func TestRenameKeyNotFound(t *testing.T) {
	RegisterTestingT(t)

	renameNode := &core.Node{ID: "rename-1", Type: "common/data_rename", Data: &core.NodeData{ID: "rename-1"}}
	parentA := &core.Node{ID: "parent-a", Type: "test", Data: &core.NodeData{ID: "parent-a"}}

	edges := []*core.Edge{
		{ID: "e1", Source: "parent-a", Target: "rename-1"},
	}

	results := map[string]map[string]interface{}{
		"parent-a": {"branch": "main"},
	}

	flow := buildTestFlow([]*core.Node{renameNode, parentA}, edges, results)

	inputs := []*core.Connection{
		{Name: "input_key", Type: core.ConnectionTypeString, Value: "nonexistent"},
		{Name: "output_key", Type: core.ConnectionTypeString, Value: "path"},
	}

	output, err := Execute(flow, renameNode, inputs)
	Expect(err).To(BeNil())
	Expect(output).To(HaveKey("path"))
	Expect(output["path"]).To(BeNil())
}

func TestRenameMissingInputKey(t *testing.T) {
	RegisterTestingT(t)

	renameNode := &core.Node{ID: "rename-1", Type: "common/data_rename", Data: &core.NodeData{ID: "rename-1"}}

	flow := buildTestFlow([]*core.Node{renameNode}, nil, nil)

	inputs := []*core.Connection{
		{Name: "input_key", Type: core.ConnectionTypeString, Value: ""},
		{Name: "output_key", Type: core.ConnectionTypeString, Value: "path"},
	}

	_, err := Execute(flow, renameNode, inputs)
	Expect(err).NotTo(BeNil())
	Expect(err.Error()).To(ContainSubstring("input_key"))
}

func TestRenameMissingOutputKey(t *testing.T) {
	RegisterTestingT(t)

	renameNode := &core.Node{ID: "rename-1", Type: "common/data_rename", Data: &core.NodeData{ID: "rename-1"}}

	flow := buildTestFlow([]*core.Node{renameNode}, nil, nil)

	inputs := []*core.Connection{
		{Name: "input_key", Type: core.ConnectionTypeString, Value: "repository_path"},
		{Name: "output_key", Type: core.ConnectionTypeString, Value: ""},
	}

	_, err := Execute(flow, renameNode, inputs)
	Expect(err).NotTo(BeNil())
	Expect(err.Error()).To(ContainSubstring("output_key"))
}
