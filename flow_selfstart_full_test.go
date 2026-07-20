package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

// buildSelfStart reconstructs the full AWS Self Start topology (all 32
// nodes/edges from the live revision) with per-node run counters, and runs it
// through the production Execute path. If1(b092088a)=true (Run Instances path);
// while(0cc7b1ad)=false (0 iterations). if2Result controls If2(f750988f): false =
// "SG already exists" → the object_get false-branch feeds Run Instances; true =
// "create a new SG" → Create Security Group + its Authorize-Ingress child run.
//
// Run Instances' security-group input references ${create-sg.group_id}, exactly
// as the real flow does — a reference to a node on a disabled branch must resolve
// to empty (never execute it).
func buildSelfStart(runs map[string]int, if2Result bool) (*Flow, map[string]Action, string) {
	mk := func(extra map[string]interface{}) Action {
		return func(f *Flow, n *Node, in []*Connection) (map[string]interface{}, error) {
			runs[n.ID]++
			out := map[string]interface{}{"ok": true}
			for k, v := range extra {
				out[k] = v
			}
			return out, nil
		}
	}
	condTrue := mk(map[string]interface{}{"result": true})
	condFalse := mk(map[string]interface{}{"result": false})
	if2 := condFalse
	if if2Result {
		if2 = condTrue
	}

	actions := map[string]Action{
		"a186e891-ee43-4de9-8a78-5e7badcef6a4": mk(nil),   // trigger/form
		"9dc5d617-89ae-4646-b715-c1826c812884": mk(nil),   // trigger/manual
		"2606d72c-6a3d-4bdd-89d3-30a52210b619": mk(nil),   // describe_instances
		"b092088a-937c-4fa0-a795-8751703b8fa5": condTrue,  // If1 -> true (Run Instances)
		"3b1fda60-1b58-43d9-bf9a-178cd911e7fd": mk(nil),   // run_instances
		"c3213622-7eae-44d2-a9b9-633bc9fa9d52": mk(nil),   // array_index
		"9f656857-6b19-4304-b025-33bad6493d8a": mk(nil),   // create_tags
		"2617d842-d670-47b5-8533-f349113733e1": mk(nil),   // describe_instances
		"9bf0b858-e8be-4691-bc8a-ecd27b40c0c8": mk(nil),   // array_index
		"b904a6b8-2110-424a-947e-aed317bb05df": mk(nil),   // object_get
		"a9bcb880-c67d-4e42-b6a7-4b40e428222d": mk(nil),   // set_variable
		"7e7bed1f-dd2b-45f6-8f07-bee1e80a066d": mk(nil),   // output/set
		"5aaccf5a-8650-4548-9026-ba0a991a078d": mk(nil),   // set_variable
		"0cc7b1ad-c7cc-4905-a6db-430f3679bcf8": condFalse, // while -> 0 iterations
		"44c75570-e33a-418a-8659-bab7c74145af": mk(nil),
		"8a1ca2dd-3ba6-4f79-94ed-fbc1ec2a8a5d": mk(nil),
		"b3ed75be-15a4-439e-a815-8810a58284ba": mk(nil),
		"78285712-2b86-4de0-b0c7-be171e1b5997": mk(nil),
		"21d962bf-c2eb-42c8-98d0-f5bc858376ce": mk(nil), // slack
		"f6132576-f2bb-4c84-9f72-b24b3b8c2c11": mk(nil), // array_index (If1 false)
		"49da5c68-6cb9-4819-a374-5f520ac9feb9": mk(nil), // object_get
		"a59917c7-7d96-4602-abda-1fe010d1925a": mk(nil),
		"da2bee64-f382-48ca-9220-91d310fa5fca": mk(nil), // object_get
		"55703d11-cf57-465d-a55c-1e3d816c5393": mk(nil), // start_instances
		"a3ac7bc1-ec98-4189-bd47-0f27241dc970": mk(nil), // slack
		"87bed1a9-8dc7-43b6-bba7-24f0d64a6ec4": mk(nil), // output/set
		"a6722aba-57c2-4b5a-8db0-12eae954781d": mk(nil), // describe_security_groups
		"f750988f-2c6e-4c63-9326-05ea0a9ce4d2": if2,     // If2 (branch under test)
		"8b3867a9-bb22-4cae-a2a3-6ce9dbb82b12": mk(nil), // create_security_group
		"c003d862-e610-4c8a-a442-834c037d8499": mk(nil), // authorize
		"8e0ff034-b10d-494e-a3c5-b40b581da4c1": mk(nil), // array_index (If2 false)
		"9ec8eaf0-4f80-4f0f-a113-9374bc621fe1": mk(nil), // object_get (If2 false)
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "a186e891-ee43-4de9-8a78-5e7badcef6a4", Type: "a186e891-ee43-4de9-8a78-5e7badcef6a4", Data: &NodeData{Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "9dc5d617-89ae-4646-b715-c1826c812884", Type: "9dc5d617-89ae-4646-b715-c1826c812884", Data: &NodeData{Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "3b1fda60-1b58-43d9-bf9a-178cd911e7fd", Type: "3b1fda60-1b58-43d9-bf9a-178cd911e7fd", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction, Inputs: []*Connection{
				{Name: "security_group_ids", Type: ConnectionTypeString, Value: "${8b3867a9-bb22-4cae-a2a3-6ce9dbb82b12.group_id}"},
			}}}},
			{ID: "9f656857-6b19-4304-b025-33bad6493d8a", Type: "9f656857-6b19-4304-b025-33bad6493d8a", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "c3213622-7eae-44d2-a9b9-633bc9fa9d52", Type: "c3213622-7eae-44d2-a9b9-633bc9fa9d52", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "2617d842-d670-47b5-8533-f349113733e1", Type: "2617d842-d670-47b5-8533-f349113733e1", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "b904a6b8-2110-424a-947e-aed317bb05df", Type: "b904a6b8-2110-424a-947e-aed317bb05df", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "9bf0b858-e8be-4691-bc8a-ecd27b40c0c8", Type: "9bf0b858-e8be-4691-bc8a-ecd27b40c0c8", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "0cc7b1ad-c7cc-4905-a6db-430f3679bcf8", Type: "0cc7b1ad-c7cc-4905-a6db-430f3679bcf8", Data: &NodeData{Config: NodeConfig{Type: ActionTypeLoop}}},
			{ID: "5aaccf5a-8650-4548-9026-ba0a991a078d", Type: "5aaccf5a-8650-4548-9026-ba0a991a078d", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "44c75570-e33a-418a-8659-bab7c74145af", Type: "44c75570-e33a-418a-8659-bab7c74145af", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "8a1ca2dd-3ba6-4f79-94ed-fbc1ec2a8a5d", Type: "8a1ca2dd-3ba6-4f79-94ed-fbc1ec2a8a5d", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "b3ed75be-15a4-439e-a815-8810a58284ba", Type: "b3ed75be-15a4-439e-a815-8810a58284ba", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "78285712-2b86-4de0-b0c7-be171e1b5997", Type: "78285712-2b86-4de0-b0c7-be171e1b5997", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "21d962bf-c2eb-42c8-98d0-f5bc858376ce", Type: "21d962bf-c2eb-42c8-98d0-f5bc858376ce", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "a9bcb880-c67d-4e42-b6a7-4b40e428222d", Type: "a9bcb880-c67d-4e42-b6a7-4b40e428222d", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "2606d72c-6a3d-4bdd-89d3-30a52210b619", Type: "2606d72c-6a3d-4bdd-89d3-30a52210b619", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "b092088a-937c-4fa0-a795-8751703b8fa5", Type: "b092088a-937c-4fa0-a795-8751703b8fa5", Data: &NodeData{Config: NodeConfig{Type: ActionTypeConditional}}},
			{ID: "55703d11-cf57-465d-a55c-1e3d816c5393", Type: "55703d11-cf57-465d-a55c-1e3d816c5393", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "f6132576-f2bb-4c84-9f72-b24b3b8c2c11", Type: "f6132576-f2bb-4c84-9f72-b24b3b8c2c11", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "a3ac7bc1-ec98-4189-bd47-0f27241dc970", Type: "a3ac7bc1-ec98-4189-bd47-0f27241dc970", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "49da5c68-6cb9-4819-a374-5f520ac9feb9", Type: "49da5c68-6cb9-4819-a374-5f520ac9feb9", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "a59917c7-7d96-4602-abda-1fe010d1925a", Type: "a59917c7-7d96-4602-abda-1fe010d1925a", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "8b3867a9-bb22-4cae-a2a3-6ce9dbb82b12", Type: "8b3867a9-bb22-4cae-a2a3-6ce9dbb82b12", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "c003d862-e610-4c8a-a442-834c037d8499", Type: "c003d862-e610-4c8a-a442-834c037d8499", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "da2bee64-f382-48ca-9220-91d310fa5fca", Type: "da2bee64-f382-48ca-9220-91d310fa5fca", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "7e7bed1f-dd2b-45f6-8f07-bee1e80a066d", Type: "7e7bed1f-dd2b-45f6-8f07-bee1e80a066d", Data: &NodeData{Config: NodeConfig{Type: ActionTypeOutput}}},
			{ID: "87bed1a9-8dc7-43b6-bba7-24f0d64a6ec4", Type: "87bed1a9-8dc7-43b6-bba7-24f0d64a6ec4", Data: &NodeData{Config: NodeConfig{Type: ActionTypeOutput}}},
			{ID: "a6722aba-57c2-4b5a-8db0-12eae954781d", Type: "a6722aba-57c2-4b5a-8db0-12eae954781d", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "f750988f-2c6e-4c63-9326-05ea0a9ce4d2", Type: "f750988f-2c6e-4c63-9326-05ea0a9ce4d2", Data: &NodeData{Config: NodeConfig{Type: ActionTypeConditional}}},
			{ID: "8e0ff034-b10d-494e-a3c5-b40b581da4c1", Type: "8e0ff034-b10d-494e-a3c5-b40b581da4c1", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "9ec8eaf0-4f80-4f0f-a113-9374bc621fe1", Type: "9ec8eaf0-4f80-4f0f-a113-9374bc621fe1", Data: &NodeData{Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{Source: "3b1fda60-1b58-43d9-bf9a-178cd911e7fd", Target: "c3213622-7eae-44d2-a9b9-633bc9fa9d52"},
			{Source: "c3213622-7eae-44d2-a9b9-633bc9fa9d52", Target: "9f656857-6b19-4304-b025-33bad6493d8a"},
			{Source: "c3213622-7eae-44d2-a9b9-633bc9fa9d52", Target: "2617d842-d670-47b5-8533-f349113733e1"},
			{Source: "2617d842-d670-47b5-8533-f349113733e1", Target: "9bf0b858-e8be-4691-bc8a-ecd27b40c0c8"},
			{Source: "9bf0b858-e8be-4691-bc8a-ecd27b40c0c8", Target: "b904a6b8-2110-424a-947e-aed317bb05df"},
			{Source: "c3213622-7eae-44d2-a9b9-633bc9fa9d52", Target: "5aaccf5a-8650-4548-9026-ba0a991a078d"},
			{Source: "c3213622-7eae-44d2-a9b9-633bc9fa9d52", Target: "0cc7b1ad-c7cc-4905-a6db-430f3679bcf8"},
			{Source: "0cc7b1ad-c7cc-4905-a6db-430f3679bcf8", Target: "44c75570-e33a-418a-8659-bab7c74145af", SourceHandle: "loop"},
			{Source: "44c75570-e33a-418a-8659-bab7c74145af", Target: "8a1ca2dd-3ba6-4f79-94ed-fbc1ec2a8a5d"},
			{Source: "8a1ca2dd-3ba6-4f79-94ed-fbc1ec2a8a5d", Target: "b3ed75be-15a4-439e-a815-8810a58284ba"},
			{Source: "b3ed75be-15a4-439e-a815-8810a58284ba", Target: "78285712-2b86-4de0-b0c7-be171e1b5997"},
			{Source: "0cc7b1ad-c7cc-4905-a6db-430f3679bcf8", Target: "21d962bf-c2eb-42c8-98d0-f5bc858376ce", SourceHandle: "output"},
			{Source: "b904a6b8-2110-424a-947e-aed317bb05df", Target: "a9bcb880-c67d-4e42-b6a7-4b40e428222d"},
			{Source: "9dc5d617-89ae-4646-b715-c1826c812884", Target: "2606d72c-6a3d-4bdd-89d3-30a52210b619"},
			{Source: "a186e891-ee43-4de9-8a78-5e7badcef6a4", Target: "2606d72c-6a3d-4bdd-89d3-30a52210b619"},
			{Source: "2606d72c-6a3d-4bdd-89d3-30a52210b619", Target: "b092088a-937c-4fa0-a795-8751703b8fa5"},
			{Source: "b092088a-937c-4fa0-a795-8751703b8fa5", Target: "3b1fda60-1b58-43d9-bf9a-178cd911e7fd", SourceHandle: "true-branch"},
			{Source: "b092088a-937c-4fa0-a795-8751703b8fa5", Target: "f6132576-f2bb-4c84-9f72-b24b3b8c2c11", SourceHandle: "false-branch"},
			{Source: "55703d11-cf57-465d-a55c-1e3d816c5393", Target: "a3ac7bc1-ec98-4189-bd47-0f27241dc970"},
			{Source: "f6132576-f2bb-4c84-9f72-b24b3b8c2c11", Target: "49da5c68-6cb9-4819-a374-5f520ac9feb9"},
			{Source: "49da5c68-6cb9-4819-a374-5f520ac9feb9", Target: "a59917c7-7d96-4602-abda-1fe010d1925a"},
			{Source: "8b3867a9-bb22-4cae-a2a3-6ce9dbb82b12", Target: "c003d862-e610-4c8a-a442-834c037d8499"},
			{Source: "8b3867a9-bb22-4cae-a2a3-6ce9dbb82b12", Target: "3b1fda60-1b58-43d9-bf9a-178cd911e7fd"},
			{Source: "f6132576-f2bb-4c84-9f72-b24b3b8c2c11", Target: "da2bee64-f382-48ca-9220-91d310fa5fca"},
			{Source: "da2bee64-f382-48ca-9220-91d310fa5fca", Target: "55703d11-cf57-465d-a55c-1e3d816c5393"},
			{Source: "b904a6b8-2110-424a-947e-aed317bb05df", Target: "7e7bed1f-dd2b-45f6-8f07-bee1e80a066d"},
			{Source: "49da5c68-6cb9-4819-a374-5f520ac9feb9", Target: "87bed1a9-8dc7-43b6-bba7-24f0d64a6ec4"},
			{Source: "a6722aba-57c2-4b5a-8db0-12eae954781d", Target: "f750988f-2c6e-4c63-9326-05ea0a9ce4d2"},
			{Source: "f750988f-2c6e-4c63-9326-05ea0a9ce4d2", Target: "8b3867a9-bb22-4cae-a2a3-6ce9dbb82b12", SourceHandle: "true-branch"},
			{Source: "f750988f-2c6e-4c63-9326-05ea0a9ce4d2", Target: "8e0ff034-b10d-494e-a3c5-b40b581da4c1", SourceHandle: "false-branch"},
			{Source: "8e0ff034-b10d-494e-a3c5-b40b581da4c1", Target: "9ec8eaf0-4f80-4f0f-a113-9374bc621fe1"},
			{Source: "9ec8eaf0-4f80-4f0f-a113-9374bc621fe1", Target: "3b1fda60-1b58-43d9-bf9a-178cd911e7fd"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}
	return f, actions, "9dc5d617-89ae-4646-b715-c1826c812884" // manual trigger entry
}

// node id aliases for readability in assertions
const (
	nRunInstances = "3b1fda60-1b58-43d9-bf9a-178cd911e7fd"
	nDescribeSG   = "a6722aba-57c2-4b5a-8db0-12eae954781d"
	nIf2          = "f750988f-2c6e-4c63-9326-05ea0a9ce4d2"
	nCreateSG     = "8b3867a9-bb22-4cae-a2a3-6ce9dbb82b12"
	nAuthorize    = "c003d862-e610-4c8a-a442-834c037d8499"
	nObjGetFalse  = "9ec8eaf0-4f80-4f0f-a113-9374bc621fe1" // object_get on If2 false-branch
)

// If2 = false ("SG already exists"): the false-branch (describe_sg -> If2 ->
// object_get) feeds Run Instances and runs; the unmatched true-branch (Create SG
// -> Authorize) is skipped; the ${create-sg.group_id} reference resolves empty.
func TestSelfStart_FalseBranch(t *testing.T) {
	RegisterTestingT(t)
	runs := map[string]int{}
	f, actions, entry := buildSelfStart(runs, false)
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())
	Expect(runs[nDescribeSG]).To(Equal(1), "describe_security_groups must run (input provider)")
	Expect(runs[nIf2]).To(Equal(1), "If2 must run")
	Expect(runs[nObjGetFalse]).To(Equal(1), "object_get (matched false-branch) must run")
	Expect(runs[nCreateSG]).To(Equal(0), "create_security_group (unmatched true-branch) must NOT run")
	Expect(runs[nAuthorize]).To(Equal(0), "authorize must NOT run")
	Expect(runs[nRunInstances]).To(Equal(1), "run_instances must run exactly once")
}

// If2 = true ("create a new SG"): the true-branch runs fully — Create SG AND its
// child Authorize-Ingress; the unmatched false-branch (object_get) is skipped;
// Run Instances runs exactly once.
func TestSelfStart_TrueBranch(t *testing.T) {
	RegisterTestingT(t)
	runs := map[string]int{}
	f, actions, entry := buildSelfStart(runs, true)
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())
	Expect(runs[nDescribeSG]).To(Equal(1), "describe_security_groups must run")
	Expect(runs[nIf2]).To(Equal(1), "If2 must run")
	Expect(runs[nCreateSG]).To(Equal(1), "create_security_group (matched true-branch) must run")
	Expect(runs[nAuthorize]).To(Equal(1), "authorize (child of matched Create SG) must run")
	Expect(runs[nObjGetFalse]).To(Equal(0), "object_get (unmatched false-branch) must NOT run")
	Expect(runs[nRunInstances]).To(Equal(1), "run_instances must run exactly once")
}
