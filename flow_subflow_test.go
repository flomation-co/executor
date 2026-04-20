package core

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

func TestSubFlow_BasicInvokeAndReturn(t *testing.T) {
	RegisterTestingT(t)

	// Main flow: trigger → invoke("double") → capture result
	// Sub-flow: begin("double") → doubler → end

	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"value": "hello"}, nil
	}

	doublerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"doubled": "hellohello"}, nil
	}

	beginAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"name": "double"}, nil
	}

	endAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "hellohello"}, nil
	}

	invokeAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{
			SubFlowNameKey: "double",
			"success":      true,
		}, nil
	}

	captureAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"captured": true}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "invoke", Type: "subflow/invoke", Data: &NodeData{Label: "subflow/invoke", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "capture", Type: "test/capture", Data: &NodeData{Label: "test/capture", Config: NodeConfig{Type: ActionTypeAction}}},
			// Sub-flow nodes (not connected to main flow via edges)
			{ID: "begin", Type: "subflow/begin", Data: &NodeData{Label: "subflow/begin", Config: NodeConfig{
				Type: ActionTypeAction,
				Inputs: []*Connection{{Name: "name", Type: ConnectionTypeString, Value: "double"}},
			}}},
			{ID: "doubler", Type: "test/doubler", Data: &NodeData{Label: "test/doubler", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "end", Type: "subflow/end", Data: &NodeData{Label: "subflow/end", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			// Main flow
			{ID: "e1", Source: "trigger", Target: "invoke"},
			{ID: "e2", Source: "invoke", Target: "capture"},
			// Sub-flow chain
			{ID: "e3", Source: "begin", Target: "doubler"},
			{ID: "e4", Source: "doubler", Target: "end"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": triggerAction,
		"subflow/invoke": invokeAction,
		"subflow/begin":  beginAction,
		"subflow/end":    endAction,
		"test/doubler":   doublerAction,
		"test/capture":   captureAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil())

	// The invoke node should have the sub-flow's outputs
	invokeResults := f.nodeResults["invoke"]
	Expect(invokeResults).ToNot(BeNil())
	Expect(invokeResults["result"]).To(Equal("hellohello"))

	// The capture node should have executed
	_, captureRan := f.nodeResults["capture"]
	Expect(captureRan).To(BeTrue())

	// The begin node should NOT have been reached by normal traversal
	// (it was invoked by the engine, not by BFS from trigger)
}

func TestSubFlow_BeginNotExecutedInNormalTraversal(t *testing.T) {
	RegisterTestingT(t)

	// If Begin Sub-Flow is somehow a child of a normal node,
	// it should be filtered out and not executed.

	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	beginAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"should_not_run": true}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "begin", Type: "subflow/begin", Data: &NodeData{Label: "subflow/begin", Config: NodeConfig{
				Type:   ActionTypeAction,
				Inputs: []*Connection{{Name: "name", Type: ConnectionTypeString, Value: "test"}},
			}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "begin"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": triggerAction,
		"subflow/begin":  beginAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil())

	// Begin should NOT have been executed
	_, beginRan := f.nodeResults["begin"]
	Expect(beginRan).To(BeFalse())
}

func TestSubFlow_MissingSubFlowReturnsError(t *testing.T) {
	RegisterTestingT(t)

	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	invokeAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{SubFlowNameKey: "nonexistent"}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "invoke", Type: "subflow/invoke", Data: &NodeData{Label: "subflow/invoke", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "invoke"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": triggerAction,
		"subflow/invoke": invokeAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("not found"))
}

func TestSubFlow_RecursionDepthLimit(t *testing.T) {
	RegisterTestingT(t)

	// Create a sub-flow that invokes itself
	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	invokeAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{SubFlowNameKey: "recursive"}, nil
	}
	beginAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"name": "recursive"}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "invoke1", Type: "subflow/invoke", Data: &NodeData{Label: "subflow/invoke", Config: NodeConfig{Type: ActionTypeAction}}},
			// Sub-flow that calls itself
			{ID: "begin", Type: "subflow/begin", Data: &NodeData{Label: "subflow/begin", Config: NodeConfig{
				Type:   ActionTypeAction,
				Inputs: []*Connection{{Name: "name", Type: ConnectionTypeString, Value: "recursive"}},
			}}},
			{ID: "invoke2", Type: "subflow/invoke", Data: &NodeData{Label: "subflow/invoke", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "invoke1"},
			{ID: "e2", Source: "begin", Target: "invoke2"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": triggerAction,
		"subflow/invoke": invokeAction,
		"subflow/begin":  beginAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("recursion depth"))
	fmt.Println("Recursion error:", err)
}

func TestSubFlow_EndNodeStopsTraversal(t *testing.T) {
	RegisterTestingT(t)

	// Nodes after End Sub-Flow should NOT execute

	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	invokeAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{SubFlowNameKey: "test"}, nil
	}
	beginAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"name": "test"}, nil
	}
	endAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"returned": "value"}, nil
	}
	shouldNotRun := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"bad": true}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "invoke", Type: "subflow/invoke", Data: &NodeData{Label: "subflow/invoke", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "begin", Type: "subflow/begin", Data: &NodeData{Label: "subflow/begin", Config: NodeConfig{
				Type:   ActionTypeAction,
				Inputs: []*Connection{{Name: "name", Type: ConnectionTypeString, Value: "test"}},
			}}},
			{ID: "end", Type: "subflow/end", Data: &NodeData{Label: "subflow/end", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "after-end", Type: "test/bad", Data: &NodeData{Label: "test/bad", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "invoke"},
			{ID: "e2", Source: "begin", Target: "end"},
			{ID: "e3", Source: "end", Target: "after-end"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": triggerAction,
		"subflow/invoke": invokeAction,
		"subflow/begin":  beginAction,
		"subflow/end":    endAction,
		"test/bad":       shouldNotRun,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil())

	// End node ran
	_, endRan := f.nodeResults["end"]
	Expect(endRan).To(BeTrue())

	// Node after End should NOT have run
	_, afterEndRan := f.nodeResults["after-end"]
	Expect(afterEndRan).To(BeFalse())
}
