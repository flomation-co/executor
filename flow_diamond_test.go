package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

// Simple diamond across an unmatched conditional branch — the conditional is an
// ancestor of C on BOTH sides, so it is always cached before C resolves A.
//
//	cond(false) --true--> A(unmatched) --\
//	             \--false--> B(matched) ---> C
func TestDiamond_UnmatchedParentNotExecuted_ChildRunsOnce(t *testing.T) {
	RegisterTestingT(t)

	var aRuns, bRuns, cRuns int
	actions := diamondActions(&aRuns, &bRuns, &cRuns)

	f := &Flow{
		Nodes: []*Node{
			{ID: "cond", Type: "conditional/if", Data: &NodeData{Config: NodeConfig{Type: ActionTypeConditional}}},
			{ID: "A", Type: "a-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "B", Type: "b-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "C", Type: "c-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "cond", Target: "A", SourceHandle: "true-branch"},
			{ID: "e2", Source: "cond", Target: "B", SourceHandle: "false-branch"},
			{ID: "e3", Source: "A", Target: "C"},
			{ID: "e4", Source: "B", Target: "C"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	_, err := f.ExecuteNode(actions, f.Nodes[0], nil)
	Expect(err).To(BeNil())
	Expect(aRuns).To(Equal(0), "A is on the unmatched branch and must not execute")
	Expect(bRuns).To(Equal(1))
	Expect(cRuns).To(Equal(1), "C must execute exactly once")
}

// The real-world topology: C is reachable via a PARALLEL path (P1) that is
// traversed BEFORE the conditional gating A has executed. When C resolves its
// parent A, the conditional is not yet cached — so the branch-skip must still
// recognise A as unmatched (by evaluating the conditional on demand).
//
//	E --> P1 --------------------> C
//	 \--> P2 --> cond(false) --true--> A --/
func TestDiamond_ParallelPath_UnmatchedParentBeforeConditional(t *testing.T) {
	RegisterTestingT(t)

	var aRuns, bRuns, cRuns int
	actions := diamondActions(&aRuns, &bRuns, &cRuns)
	actions["e-action"] = func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"e": "ran"}, nil
	}
	actions["p-action"] = func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"p": "ran"}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "E", Type: "e-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "P1", Type: "p-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "P2", Type: "p-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "cond", Type: "conditional/if", Data: &NodeData{Config: NodeConfig{Type: ActionTypeConditional}}},
			{ID: "A", Type: "a-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "C", Type: "c-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "E", Target: "P1"},
			{ID: "e2", Source: "E", Target: "P2"},
			{ID: "e3", Source: "P1", Target: "C"},
			{ID: "e4", Source: "P2", Target: "cond"},
			{ID: "e5", Source: "cond", Target: "A", SourceHandle: "true-branch"},
			{ID: "e6", Source: "A", Target: "C"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	_, err := f.ExecuteNode(actions, f.Nodes[0], nil)
	Expect(err).To(BeNil())
	Expect(aRuns).To(Equal(0), "A is on the unmatched branch; it must not execute even when C is resolved before the conditional runs")
	Expect(cRuns).To(Equal(1), "C must execute exactly once regardless of route")
}

func diamondActions(aRuns, bRuns, cRuns *int) map[string]Action {
	return map[string]Action{
		"conditional/if": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			return map[string]interface{}{"result": false}, nil
		},
		"a-action": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			*aRuns++
			return map[string]interface{}{"a": "ran"}, nil
		},
		"b-action": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			*bRuns++
			return map[string]interface{}{"b": "ran"}, nil
		},
		"c-action": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			*cRuns++
			return map[string]interface{}{"c": "ran"}, nil
		},
	}
}

// Real-world reproduction (AWS Self Start flow): the shared child C is fed by a
// MATCHED parent M that lives on a subgraph only connected to the main flow via
// C itself (M is never forward-reached from the entry). Resolving M as C's input
// must not cascade forward into C and re-run it. A, on the unmatched branch of
// the same gating conditional, must not run at all.
//
//	entry --true--> C  (C is reached here)
//	root  --> cond(false) --true--> A(unmatched) --> C
//	                       \--false--> M(matched) --> C
func TestDiamond_MatchedParentNotForwardReached_NoDoubleRun(t *testing.T) {
	RegisterTestingT(t)

	var aRuns, cRuns, mRuns, condRuns int
	actions := map[string]Action{
		"entry-cond": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			return map[string]interface{}{"result": true}, nil // routes to C
		},
		"gate-cond": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			condRuns++
			return map[string]interface{}{"result": false}, nil // A unmatched, M matched
		},
		"root": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			return map[string]interface{}{"root": "ran"}, nil
		},
		"a-action": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			aRuns++
			return map[string]interface{}{"a": "ran"}, nil
		},
		"m-action": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			mRuns++
			return map[string]interface{}{"m": "ran"}, nil
		},
		"c-action": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			cRuns++
			return map[string]interface{}{"c": "ran"}, nil
		},
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "entry", Type: "entry-cond", Data: &NodeData{Config: NodeConfig{Type: ActionTypeConditional}}},
			{ID: "root", Type: "root", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "gate", Type: "gate-cond", Data: &NodeData{Config: NodeConfig{Type: ActionTypeConditional}}},
			{ID: "A", Type: "a-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "M", Type: "m-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "C", Type: "c-action", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "entry", Target: "C", SourceHandle: "true-branch"},
			{ID: "e2", Source: "root", Target: "gate"},
			{ID: "e3", Source: "gate", Target: "A", SourceHandle: "true-branch"},
			{ID: "e4", Source: "gate", Target: "M", SourceHandle: "false-branch"},
			{ID: "e5", Source: "A", Target: "C"},
			{ID: "e6", Source: "M", Target: "C"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	// entry is the trigger-side start; root feeds the gate subgraph. Execute
	// from entry (as the engine would from the reachable trigger path).
	_, err := f.ExecuteNode(actions, f.Nodes[0], nil)
	Expect(err).To(BeNil())
	Expect(aRuns).To(Equal(0), "A is on the unmatched branch — must not execute")
	Expect(cRuns).To(Equal(1), "C must execute exactly once, not re-run by a parent's forward cascade")
}
