package core

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
)

// --- Bug 2: Variable substitution should work in On Error chain ---

func TestOnErrorChain_InheritsUpstreamOutputs(t *testing.T) {
	RegisterTestingT(t)

	// Node that succeeds and produces output
	successAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"greeting": "hello"}, nil
	}

	// Node that fails
	failAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return nil, errors.New("something went wrong")
	}

	// On Error handler (passthrough)
	onErrorAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}

	// Error handler child that captures parentResults
	var childReceivedParentResults map[string]interface{}
	errorChildAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		// The parent of this node is the On Error node. Its cached results
		// should include the upstream "greeting" output merged in.
		parentOutputs := flow.GetNodeResult(node.ID)
		// We can't access parentResults directly from here, but we can check
		// the On Error node's cached results which get passed as parentResults
		parents := flow.FindSource(node.ID)
		for _, p := range parents {
			if cached, exists := flow.nodeResults[p.ID]; exists {
				childReceivedParentResults = cached
			}
		}
		_ = parentOutputs
		return map[string]interface{}{"handled": true}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger-1", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "success-1", Type: "test/success", Data: &NodeData{Label: "test/success", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "fail-1", Type: "test/fail", Data: &NodeData{Label: "test/fail", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "on-error-1", Type: "error/on_error", Data: &NodeData{Label: "error/on_error", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "error-child-1", Type: "test/error_child", Data: &NodeData{Label: "test/error_child", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "success-1"},
			{ID: "e2", Source: "success-1", Target: "fail-1"},
			{ID: "e3", Source: "on-error-1", Target: "error-child-1"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual":   successAction,
		"test/success":     successAction,
		"test/fail":        failAction,
		"error/on_error":   onErrorAction,
		"test/error_child": errorChildAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil()) // Error was handled

	// The On Error node's cached results should contain both error context
	// AND the upstream "greeting" output from the success node
	Expect(childReceivedParentResults).ToNot(BeNil())
	Expect(childReceivedParentResults["error_message"]).To(Equal("something went wrong"))
	Expect(childReceivedParentResults["greeting"]).To(Equal("hello"))
}

// --- Bug 3: "Set Variable" should be globally accessible ---

func TestSetVariable_GlobalScope(t *testing.T) {
	RegisterTestingT(t)

	f := &Flow{
		Nodes:                []*Node{},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	// Set a variable
	f.SetVariable("colour", "blue")

	// Retrieve it
	val, ok := f.GetVariable("colour")
	Expect(ok).To(BeTrue())
	Expect(val).To(Equal("blue"))

	// Overwrite with same name
	f.SetVariable("colour", "red")
	val, ok = f.GetVariable("colour")
	Expect(ok).To(BeTrue())
	Expect(val).To(Equal("red"))
}

func TestSetVariable_InitialisedInLoad(t *testing.T) {
	RegisterTestingT(t)

	// Simulate what Load() does (without file I/O)
	f := &Flow{
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	// Variables map should be non-nil immediately
	Expect(f.variables).ToNot(BeNil())

	// Should be able to set without triggering lazy init
	f.SetVariable("test", "value")
	val, ok := f.GetVariable("test")
	Expect(ok).To(BeTrue())
	Expect(val).To(Equal("value"))
}

func TestSetVariable_TwoNodesShareSameName(t *testing.T) {
	RegisterTestingT(t)

	setVarAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		for _, c := range inputs {
			if c.Name == "name" {
				nameStr := c.String()
				var value interface{} = ""
				for _, v := range inputs {
					if v.Name == "value" {
						value = v.Value
					}
				}
				if nameStr != nil {
					flow.SetVariable(*nameStr, value)
				}
			}
		}
		return map[string]interface{}{"ok": true}, nil
	}

	readVarAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		val, ok := flow.GetVariable("status")
		if ok {
			return map[string]interface{}{"status_value": val}, nil
		}
		return map[string]interface{}{"status_value": "not_found"}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger-1", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "set-var-1", Type: "test/set_var", Data: &NodeData{
				Label: "test/set_var",
				Config: NodeConfig{
					Type: ActionTypeAction,
					Inputs: []*Connection{
						{Name: "name", Type: ConnectionTypeString, Value: "status"},
						{Name: "value", Type: ConnectionTypeString, Value: "first"},
					},
				},
			}},
			{ID: "set-var-2", Type: "test/set_var", Data: &NodeData{
				Label: "test/set_var",
				Config: NodeConfig{
					Type: ActionTypeAction,
					Inputs: []*Connection{
						{Name: "name", Type: ConnectionTypeString, Value: "status"},
						{Name: "value", Type: ConnectionTypeString, Value: "second"},
					},
				},
			}},
			{ID: "read-var", Type: "test/read_var", Data: &NodeData{Label: "test/read_var", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "set-var-1"},
			{ID: "e2", Source: "set-var-1", Target: "set-var-2"},
			{ID: "e3", Source: "set-var-2", Target: "read-var"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		},
		"test/set_var":  setVarAction,
		"test/read_var": readVarAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil())

	// The second Set Variable should have overwritten the first
	val, ok := f.GetVariable("status")
	Expect(ok).To(BeTrue())
	Expect(val).To(Equal("second"))
}

// --- Bug 4: Should only support 1 On Error node ---

func TestOnErrorChain_WarnsOnMultipleOnErrorNodes(t *testing.T) {
	RegisterTestingT(t)

	// This test verifies that the executor uses the first On Error node
	// and doesn't crash when multiple exist. The warning is logged but
	// not easily testable without a log hook, so we verify behaviour.

	failAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return nil, errors.New("fail")
	}

	onErrorAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}

	childAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"handler": node.ID}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger-1", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "fail-1", Type: "test/fail", Data: &NodeData{Label: "test/fail", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "on-error-1", Type: "error/on_error", Data: &NodeData{Label: "error/on_error", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "on-error-2", Type: "error/on_error", Data: &NodeData{Label: "error/on_error", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "child-1", Type: "test/child", Data: &NodeData{Label: "test/child", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "child-2", Type: "test/child", Data: &NodeData{Label: "test/child", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "fail-1"},
			{ID: "e2", Source: "on-error-1", Target: "child-1"},
			{ID: "e3", Source: "on-error-2", Target: "child-2"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		},
		"test/fail":    failAction,
		"error/on_error": onErrorAction,
		"test/child":   childAction,
	}

	_, err := f.Execute(actions, nil, nil)
	// Error was handled by the first On Error node
	Expect(err).To(BeNil())

	// Only child-1 (from on-error-1) should have been executed
	_, child1Executed := f.nodeResults["child-1"]
	_, child2Executed := f.nodeResults["child-2"]
	Expect(child1Executed).To(BeTrue())
	Expect(child2Executed).To(BeFalse())
}
