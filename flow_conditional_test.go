package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestFindTargetByHandle(t *testing.T) {
	RegisterTestingT(t)

	f := &Flow{
		Nodes: []*Node{
			{ID: "node-1", Type: "conditional/if", Data: &NodeData{}},
			{ID: "node-2", Type: "action", Data: &NodeData{}},
			{ID: "node-3", Type: "action", Data: &NodeData{}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "node-1", Target: "node-2", SourceHandle: "true-branch"},
			{ID: "e2", Source: "node-1", Target: "node-3", SourceHandle: "false-branch"},
		},
	}

	trueTargets := f.FindTargetByHandle("node-1", "true-branch")
	Expect(trueTargets).To(HaveLen(1))
	Expect(trueTargets[0].ID).To(Equal("node-2"))

	falseTargets := f.FindTargetByHandle("node-1", "false-branch")
	Expect(falseTargets).To(HaveLen(1))
	Expect(falseTargets[0].ID).To(Equal("node-3"))

	noTargets := f.FindTargetByHandle("node-1", "nonexistent")
	Expect(noTargets).To(HaveLen(0))
}

func TestFindTargetByHandle_NilEdge(t *testing.T) {
	RegisterTestingT(t)

	f := &Flow{
		Nodes: []*Node{
			{ID: "node-1", Type: "conditional/if", Data: &NodeData{}},
		},
		Edges: []*Edge{nil},
	}

	targets := f.FindTargetByHandle("node-1", "true-branch")
	Expect(targets).To(HaveLen(0))
}

func TestExecuteNode_ConditionalBranching_True(t *testing.T) {
	RegisterTestingT(t)

	trueAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"branch": "true"}, nil
	}
	falseAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"branch": "false"}, nil
	}
	condAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"result": true}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "cond-1",
				Type: "conditional/if",
				Data: &NodeData{
					Config: NodeConfig{Type: ActionTypeConditional},
				},
			},
			{
				ID:   "true-node",
				Type: "true-action",
				Data: &NodeData{
					Config: NodeConfig{Type: ActionTypeAction},
				},
			},
			{
				ID:   "false-node",
				Type: "false-action",
				Data: &NodeData{
					Config: NodeConfig{Type: ActionTypeAction},
				},
			},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "cond-1", Target: "true-node", SourceHandle: "true-branch"},
			{ID: "e2", Source: "cond-1", Target: "false-node", SourceHandle: "false-branch"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	actions := map[string]Action{
		"conditional/if": condAction,
		"true-action":    trueAction,
		"false-action":   falseAction,
	}

	results, err := f.ExecuteNode(actions, f.Nodes[0], nil)
	Expect(err).To(BeNil())
	Expect(results["branch"]).To(Equal("true"))
}

func TestExecuteNode_ConditionalBranching_False(t *testing.T) {
	RegisterTestingT(t)

	trueAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"branch": "true"}, nil
	}
	falseAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"branch": "false"}, nil
	}
	condAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"result": false}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "cond-1",
				Type: "conditional/if",
				Data: &NodeData{
					Config: NodeConfig{Type: ActionTypeConditional},
				},
			},
			{
				ID:   "true-node",
				Type: "true-action",
				Data: &NodeData{
					Config: NodeConfig{Type: ActionTypeAction},
				},
			},
			{
				ID:   "false-node",
				Type: "false-action",
				Data: &NodeData{
					Config: NodeConfig{Type: ActionTypeAction},
				},
			},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "cond-1", Target: "true-node", SourceHandle: "true-branch"},
			{ID: "e2", Source: "cond-1", Target: "false-node", SourceHandle: "false-branch"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	actions := map[string]Action{
		"conditional/if": condAction,
		"true-action":    trueAction,
		"false-action":   falseAction,
	}

	results, err := f.ExecuteNode(actions, f.Nodes[0], nil)
	Expect(err).To(BeNil())
	Expect(results["branch"]).To(Equal("false"))
}
