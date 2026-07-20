package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

// Reproduces the AWS Self Start topology via the PRODUCTION entry point
// (Flow.Execute from the trigger), so reachableNodes is computed exactly as in a
// real run — unlike the ExecuteNode-direct tests which leave it nil.
//
//	T(trigger) -> DI -> If1 --true--> RI(run_instances)
//	DSG(root, no trigger edge) -> If2 --true--> CSG(create_sg) -> RI, and -> AUTH
//	                                  \--false--> OG(object_get) -> RI
//
// If2 routes false, so CSG is on the unmatched branch. CSG/OG/If2/DSG are NOT
// forward-reachable from the trigger. Expected: CSG and AUTH never run; RI runs
// exactly once.
func TestSelfStart_Repro_ViaExecute(t *testing.T) {
	RegisterTestingT(t)

	var diRuns, riRuns, csgRuns, authRuns, ogRuns, dsgRuns int
	actions := map[string]Action{
		"trigger/manual": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		},
		"di": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			diRuns++
			return map[string]interface{}{"di": "ran"}, nil
		},
		"if1": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			return map[string]interface{}{"result": true}, nil
		},
		"if2": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			return map[string]interface{}{"result": false}, nil
		},
		"dsg": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			dsgRuns++
			return map[string]interface{}{"dsg": "ran"}, nil
		},
		"ri": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			riRuns++
			return map[string]interface{}{"ri": "ran"}, nil
		},
		"csg": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			csgRuns++
			return map[string]interface{}{"csg": "ran"}, nil
		},
		"auth": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			authRuns++
			return map[string]interface{}{"auth": "ran"}, nil
		},
		"og": func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			ogRuns++
			return map[string]interface{}{"og": "ran"}, nil
		},
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "T", Type: "trigger/manual", Data: &NodeData{Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "DI", Type: "di", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "If1", Type: "if1", Data: &NodeData{Config: NodeConfig{Type: ActionTypeConditional}}},
			{ID: "RI", Type: "ri", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "DSG", Type: "dsg", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "If2", Type: "if2", Data: &NodeData{Config: NodeConfig{Type: ActionTypeConditional}}},
			{ID: "CSG", Type: "csg", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "AUTH", Type: "auth", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "OG", Type: "og", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "T", Target: "DI"},
			{ID: "e2", Source: "DI", Target: "If1"},
			{ID: "e3", Source: "If1", Target: "RI", SourceHandle: "true-branch"},
			{ID: "e4", Source: "DSG", Target: "If2"},
			{ID: "e5", Source: "If2", Target: "CSG", SourceHandle: "true-branch"},
			{ID: "e6", Source: "CSG", Target: "AUTH"},
			{ID: "e7", Source: "CSG", Target: "RI"},
			{ID: "e8", Source: "If2", Target: "OG", SourceHandle: "false-branch"},
			{ID: "e9", Source: "OG", Target: "RI"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	entry := "T"
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())
	t.Logf("DI=%d If1->RI riRuns=%d CSG=%d AUTH=%d OG=%d DSG=%d", diRuns, riRuns, csgRuns, authRuns, ogRuns, dsgRuns)
	Expect(csgRuns).To(Equal(0), "Create Security Group is unreachable + unmatched — must not execute")
	Expect(authRuns).To(Equal(0), "Authorize (child of CSG) must not execute")
	Expect(riRuns).To(Equal(1), "Run Instances must execute exactly once")
}
