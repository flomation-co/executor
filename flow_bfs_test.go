package core

import (
	gocontext "context"
	"fmt"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
)

// TestBreadthFirstSiblingExecution verifies that all sibling nodes at the same
// depth execute their actions before any of their children are traversed.
//
// Graph:
//   trigger -> A -> A1
//           -> B -> B1
//
// Expected order: trigger, then A+B (siblings), then A1+B1 (next level).
// A and B must both execute before A1 or B1.
func TestBreadthFirstSiblingExecution(t *testing.T) {
	RegisterTestingT(t)

	var mu sync.Mutex
	var executionOrder []string

	recordAction := func(name string) Action {
		return func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			mu.Lock()
			executionOrder = append(executionOrder, name)
			mu.Unlock()
			return map[string]interface{}{"node": name}, nil
		}
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "A", Type: "action/a", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "B", Type: "action/b", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "A1", Type: "action/a1", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "B1", Type: "action/b1", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "A"},
			{ID: "e2", Source: "trigger", Target: "B"},
			{ID: "e3", Source: "A", Target: "A1"},
			{ID: "e4", Source: "B", Target: "B1"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": recordAction("trigger"),
		"action/a":       recordAction("A"),
		"action/b":       recordAction("B"),
		"action/a1":      recordAction("A1"),
		"action/b1":      recordAction("B1"),
	}

	_, err := f.ExecuteNode(actions, f.Nodes[0], nil)
	Expect(err).To(BeNil())

	// Verify execution order: trigger first
	Expect(executionOrder[0]).To(Equal("trigger"))

	// A and B must both appear before A1 and B1
	indexA := indexOf(executionOrder, "A")
	indexB := indexOf(executionOrder, "B")
	indexA1 := indexOf(executionOrder, "A1")
	indexB1 := indexOf(executionOrder, "B1")

	Expect(indexA).To(BeNumerically("<", indexA1), "A should execute before A1")
	Expect(indexA).To(BeNumerically("<", indexB1), "A should execute before B1")
	Expect(indexB).To(BeNumerically("<", indexA1), "B should execute before A1")
	Expect(indexB).To(BeNumerically("<", indexB1), "B should execute before B1")
}

// TestBreadthFirstThreeChildren verifies BFS with three siblings and their children.
//
// Graph:
//   trigger -> A -> A1
//           -> B -> B1
//           -> C -> C1
//
// Expected: trigger, then A+B+C (all siblings), then A1/B1/C1 (each after its parent)
func TestBreadthFirstThreeChildren(t *testing.T) {
	RegisterTestingT(t)

	var mu sync.Mutex
	var executionOrder []string

	recordAction := func(name string) Action {
		return func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			mu.Lock()
			executionOrder = append(executionOrder, name)
			mu.Unlock()
			return map[string]interface{}{"node": name}, nil
		}
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "A", Type: "action/a", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "B", Type: "action/b", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "C", Type: "action/c", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "A1", Type: "action/a1", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "B1", Type: "action/b1", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "C1", Type: "action/c1", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "A"},
			{ID: "e2", Source: "trigger", Target: "B"},
			{ID: "e3", Source: "trigger", Target: "C"},
			{ID: "e4", Source: "A", Target: "A1"},
			{ID: "e5", Source: "B", Target: "B1"},
			{ID: "e6", Source: "C", Target: "C1"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": recordAction("trigger"),
		"action/a":       recordAction("A"),
		"action/b":       recordAction("B"),
		"action/c":       recordAction("C"),
		"action/a1":      recordAction("A1"),
		"action/b1":      recordAction("B1"),
		"action/c1":      recordAction("C1"),
	}

	_, err := f.ExecuteNode(actions, f.Nodes[0], nil)
	Expect(err).To(BeNil())

	Expect(executionOrder[0]).To(Equal("trigger"))

	indexA := indexOf(executionOrder, "A")
	indexB := indexOf(executionOrder, "B")
	indexC := indexOf(executionOrder, "C")
	indexA1 := indexOf(executionOrder, "A1")
	indexB1 := indexOf(executionOrder, "B1")
	indexC1 := indexOf(executionOrder, "C1")

	// All siblings (A, B, C) must execute before any of their children
	Expect(indexA).To(BeNumerically("<", indexA1), "A before A1")
	Expect(indexA).To(BeNumerically("<", indexB1), "A before B1")
	Expect(indexA).To(BeNumerically("<", indexC1), "A before C1")
	Expect(indexB).To(BeNumerically("<", indexA1), "B before A1")
	Expect(indexB).To(BeNumerically("<", indexB1), "B before B1")
	Expect(indexB).To(BeNumerically("<", indexC1), "B before C1")
	Expect(indexC).To(BeNumerically("<", indexA1), "C before A1")
	Expect(indexC).To(BeNumerically("<", indexB1), "C before B1")
	Expect(indexC).To(BeNumerically("<", indexC1), "C before C1")
}

// TestChildErrorPropagatesUpward verifies that when a child node fails,
// the error propagates back to the caller rather than being swallowed.
// Regression test: a bash script timeout was not terminating the execution.
func TestChildErrorPropagatesUpward(t *testing.T) {
	RegisterTestingT(t)

	failAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return nil, fmt.Errorf("script exceeded timeout of 60 seconds")
	}
	okAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"ok": true}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "fail-node", Type: "action/fail", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "after-fail", Type: "action/ok", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "fail-node"},
			{ID: "e2", Source: "fail-node", Target: "after-fail"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": okAction,
		"action/fail":    failAction,
		"action/ok":      okAction,
	}

	_, err := f.ExecuteNode(actions, f.Nodes[0], nil)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("timeout"))
}

// TestSiblingErrorStopsExecution verifies that when one sibling fails in
// Pass 1 of BFS, remaining siblings and Pass 2 are not executed.
func TestSiblingErrorStopsExecution(t *testing.T) {
	RegisterTestingT(t)

	var mu sync.Mutex
	var executionOrder []string

	recordAction := func(name string) Action {
		return func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			mu.Lock()
			executionOrder = append(executionOrder, name)
			mu.Unlock()
			return map[string]interface{}{"node": name}, nil
		}
	}
	failAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		mu.Lock()
		executionOrder = append(executionOrder, "FAIL")
		mu.Unlock()
		return nil, fmt.Errorf("node failed")
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "A", Type: "action/a", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "B", Type: "action/fail", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "C", Type: "action/c", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "A1", Type: "action/a1", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "A"},
			{ID: "e2", Source: "trigger", Target: "B"},
			{ID: "e3", Source: "trigger", Target: "C"},
			{ID: "e4", Source: "A", Target: "A1"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": recordAction("trigger"),
		"action/a":       recordAction("A"),
		"action/fail":    failAction,
		"action/c":       recordAction("C"),
		"action/a1":      recordAction("A1"),
	}

	_, err := f.ExecuteNode(actions, f.Nodes[0], nil)
	Expect(err).ToNot(BeNil())

	// A executed before B (the failing node), but C and A1 should NOT have executed
	Expect(executionOrder).To(ContainElement("A"))
	Expect(executionOrder).To(ContainElement("FAIL"))
	Expect(executionOrder).ToNot(ContainElement("C"), "C should not execute after B failed")
	Expect(executionOrder).ToNot(ContainElement("A1"), "A1 should not execute after B failed")
}

// TestCancellationStopsExecution verifies that cancelling the flow's context
// prevents subsequent nodes from executing.
func TestCancellationStopsExecution(t *testing.T) {
	RegisterTestingT(t)

	var mu sync.Mutex
	var executionOrder []string

	recordAction := func(name string) Action {
		return func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			mu.Lock()
			executionOrder = append(executionOrder, name)
			mu.Unlock()
			return map[string]interface{}{"node": name}, nil
		}
	}

	// Action that cancels the flow after executing
	cancellingAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		mu.Lock()
		executionOrder = append(executionOrder, "CANCEL")
		mu.Unlock()
		flow.Cancel()
		return map[string]interface{}{"node": "cancel"}, nil
	}

	ctx, cancel := gocontext.WithCancel(gocontext.Background())
	defer cancel()

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "A", Type: "action/a", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "B", Type: "action/b", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "A"},
			{ID: "e2", Source: "A", Target: "B"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}
	f.SetCancelContext(ctx, cancel)

	actions := map[string]Action{
		"trigger/manual": recordAction("trigger"),
		"action/a":       cancellingAction,
		"action/b":       recordAction("B"),
	}

	_, err := f.ExecuteNode(actions, f.Nodes[0], nil)
	Expect(err).To(Equal(ErrCancelled))

	// Trigger and A (cancel) should have executed, but B should not
	Expect(executionOrder).To(ContainElement("trigger"))
	Expect(executionOrder).To(ContainElement("CANCEL"))
	Expect(executionOrder).ToNot(ContainElement("B"))
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
