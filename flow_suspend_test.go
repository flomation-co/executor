package core

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

// triggerPauseFinalGraph builds the canonical "trigger → Pause → final"
// shape the user is reproducing in their browser. Counters track each
// node's action invocations so the test can distinguish "ran action" from
// "hit cache + walked children" — the executor must do the latter on
// resume, never the former.
type cycleCounters struct {
	trigger int
	pause   int
	final   int
	resumed bool
}

func buildPauseGraph(c *cycleCounters) (*Flow, map[string]Action) {
	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "pause", Type: "common/pause", Data: &NodeData{Label: "common/pause", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "final", Type: "test/final", Data: &NodeData{Label: "test/final", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "pause"},
			{ID: "e2", Source: "pause", Target: "final"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			c.trigger++
			return map[string]interface{}{"seq": c.trigger}, nil
		},
		"common/pause": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			c.pause++
			// Mimic the real pause action exactly.
			if flow.IsResumedNode(node.ID) {
				c.resumed = true
				return map[string]interface{}{
					"tool_result": "Resumed",
					"suspended":   false,
					"reason":      "manual",
				}, nil
			}
			flow.Suspend(&SuspendInfo{NodeID: node.ID, Reason: "manual"})
			return map[string]interface{}{
				"tool_result": "Manual pause",
				"suspended":   true,
				"reason":      "manual",
			}, ErrSuspended
		},
		"test/final": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			c.final++
			return map[string]interface{}{"done": true}, nil
		},
	}
	return f, actions
}

// roundTripCheckpoint marshals the checkpoint through JSON, exactly as the
// API does (executor writes → state.json → API extracts → Postgres column
// → API serves → runner writes → executor reads). If anything loses
// fidelity in that round-trip, the restored flow won't behave correctly.
func roundTripCheckpoint(t *testing.T, cp *Checkpoint) *Checkpoint {
	t.Helper()
	raw, err := json.Marshal(cp)
	Expect(err).NotTo(HaveOccurred())
	var restored Checkpoint
	Expect(json.Unmarshal(raw, &restored)).To(Succeed())
	return &restored
}

// The canonical scenario from the browser screenshot: trigger → Pause →
// Set Output (here: test/final). Pause must suspend on the first run and
// pass through on resume so final fires exactly once.
func TestSuspendResume_PauseCycle_NoActionReFire(t *testing.T) {
	RegisterTestingT(t)

	c := &cycleCounters{}
	f1, actions := buildPauseGraph(c)

	_, err := f1.Execute(actions, nil, nil)
	Expect(errors.Is(err, ErrSuspended)).To(BeTrue(), "first execution should suspend at pause")
	Expect(c.trigger).To(Equal(1), "trigger ran once in original execution")
	Expect(c.pause).To(Equal(1), "pause ran once in original execution")
	Expect(c.final).To(Equal(0), "final must not run before resume")

	// Snapshot, round-trip through JSON.
	cp := f1.CreateCheckpoint()
	Expect(cp.SuspendInfo).NotTo(BeNil(), "checkpoint must carry suspend info")
	Expect(cp.SuspendInfo.NodeID).To(Equal("pause"))
	Expect(cp.NodeResults["trigger"]).NotTo(BeNil(), "trigger output must persist in checkpoint")
	cp = roundTripCheckpoint(t, cp)

	// Build a fresh Flow (mimicking a new executor process) and restore.
	f2, _ := buildPauseGraph(c)
	f2.RestoreCheckpoint(cp)
	Expect(f2.IsResumed()).To(BeTrue())
	Expect(f2.IsResumedNode("pause")).To(BeTrue())

	_, err = f2.Execute(actions, nil, nil)
	Expect(err).To(BeNil(), "resume execution should complete cleanly")

	// THIS is what the user is suspicious about: the trigger action must
	// NOT run a second time. The executor's cache-hit-but-not-traversed
	// branch should walk children without re-invoking the action.
	Expect(c.trigger).To(Equal(1), "trigger action must NOT re-run on resume — cache hit should walk children")
	Expect(c.pause).To(Equal(2), "pause action runs once more for pass-through")
	Expect(c.resumed).To(BeTrue(), "pause must take the IsResumedNode pass-through branch")
	Expect(c.final).To(Equal(1), "final must run exactly once, after resume")
}

// Wait suspends with a ResumeAt timestamp. The executor's behaviour on
// resume is identical to Pause: pass through, fire children. The only
// difference is the checkpoint carries a ResumeAt the API uses to wake
// the execution at the right time.
func TestSuspendResume_WaitCycle_PreservesResumeAt(t *testing.T) {
	RegisterTestingT(t)

	resumeAt := time.Now().Add(5 * time.Minute).UTC().Truncate(time.Second)
	c := &cycleCounters{}

	build := func() (*Flow, map[string]Action) {
		f := &Flow{
			Nodes: []*Node{
				{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
				{ID: "wait", Type: "common/wait", Data: &NodeData{Label: "common/wait", Config: NodeConfig{Type: ActionTypeAction}}},
				{ID: "final", Type: "test/final", Data: &NodeData{Label: "test/final", Config: NodeConfig{Type: ActionTypeAction}}},
			},
			Edges: []*Edge{
				{ID: "e1", Source: "trigger", Target: "wait"},
				{ID: "e2", Source: "wait", Target: "final"},
			},
			nodeResults:          make(map[string]map[string]interface{}),
			nodeExecutionResults: make(map[string]*ExecutionNodeResult),
			outputs:              make(map[string]interface{}),
			variables:            make(map[string]interface{}),
		}
		actions := map[string]Action{
			"trigger/manual": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
				c.trigger++
				return map[string]interface{}{}, nil
			},
			"common/wait": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
				c.pause++
				if flow.IsResumedNode(node.ID) {
					c.resumed = true
					return map[string]interface{}{"suspended": false}, nil
				}
				ra := resumeAt
				flow.Suspend(&SuspendInfo{NodeID: node.ID, Reason: "wait", ResumeAt: &ra})
				return map[string]interface{}{"suspended": true, "resume_at": ra.Format(time.RFC3339)}, ErrSuspended
			},
			"test/final": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
				c.final++
				return map[string]interface{}{"done": true}, nil
			},
		}
		return f, actions
	}

	f1, actions := build()
	_, err := f1.Execute(actions, nil, nil)
	Expect(errors.Is(err, ErrSuspended)).To(BeTrue())

	cp := f1.CreateCheckpoint()
	Expect(cp.SuspendInfo).NotTo(BeNil())
	Expect(cp.SuspendInfo.ResumeAt).NotTo(BeNil())
	Expect(cp.SuspendInfo.ResumeAt.Unix()).To(Equal(resumeAt.Unix()), "ResumeAt must round-trip through JSON")
	cp = roundTripCheckpoint(t, cp)
	Expect(cp.SuspendInfo.ResumeAt.Unix()).To(Equal(resumeAt.Unix()), "ResumeAt must survive JSON serialisation")

	f2, _ := build()
	f2.RestoreCheckpoint(cp)
	_, err = f2.Execute(actions, nil, nil)
	Expect(err).To(BeNil())
	Expect(c.trigger).To(Equal(1), "trigger action must NOT re-run on resume")
	Expect(c.pause).To(Equal(2))
	Expect(c.resumed).To(BeTrue())
	Expect(c.final).To(Equal(1))
}

// Edge case: when a node BEFORE Pause already produced a side effect
// (sent a Slack message, hit an API, fired a webhook), re-running it on
// resume would double-fire. This test makes the no-re-fire guarantee
// explicit by counting parent invocations through a longer chain.
func TestSuspendResume_LongerChain_NoUpstreamReExecution(t *testing.T) {
	RegisterTestingT(t)

	c := &cycleCounters{}
	preCount := 0

	build := func() (*Flow, map[string]Action) {
		f := &Flow{
			Nodes: []*Node{
				{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
				{ID: "pre", Type: "test/side_effect", Data: &NodeData{Label: "test/side_effect", Config: NodeConfig{Type: ActionTypeAction}}},
				{ID: "pause", Type: "common/pause", Data: &NodeData{Label: "common/pause", Config: NodeConfig{Type: ActionTypeAction}}},
				{ID: "final", Type: "test/final", Data: &NodeData{Label: "test/final", Config: NodeConfig{Type: ActionTypeAction}}},
			},
			Edges: []*Edge{
				{ID: "e1", Source: "trigger", Target: "pre"},
				{ID: "e2", Source: "pre", Target: "pause"},
				{ID: "e3", Source: "pause", Target: "final"},
			},
			nodeResults:          make(map[string]map[string]interface{}),
			nodeExecutionResults: make(map[string]*ExecutionNodeResult),
			outputs:              make(map[string]interface{}),
			variables:            make(map[string]interface{}),
		}
		actions := map[string]Action{
			"trigger/manual": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
				c.trigger++
				return map[string]interface{}{}, nil
			},
			"test/side_effect": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
				preCount++
				return map[string]interface{}{"sent": true}, nil
			},
			"common/pause": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
				c.pause++
				if flow.IsResumedNode(node.ID) {
					c.resumed = true
					return map[string]interface{}{"suspended": false}, nil
				}
				flow.Suspend(&SuspendInfo{NodeID: node.ID, Reason: "manual"})
				return map[string]interface{}{"suspended": true}, ErrSuspended
			},
			"test/final": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
				c.final++
				return map[string]interface{}{"done": true}, nil
			},
		}
		return f, actions
	}

	f1, actions := build()
	_, err := f1.Execute(actions, nil, nil)
	Expect(errors.Is(err, ErrSuspended)).To(BeTrue())
	Expect(preCount).To(Equal(1), "side-effect action ran once in original execution")

	cp := roundTripCheckpoint(t, f1.CreateCheckpoint())

	f2, _ := build()
	f2.RestoreCheckpoint(cp)
	_, err = f2.Execute(actions, nil, nil)
	Expect(err).To(BeNil())

	// THE KEY ASSERTION: a side-effect node upstream of pause must NEVER
	// re-fire on resume. If this fails, every Slack send / API call /
	// webhook between trigger and pause gets duplicated on each resume.
	Expect(preCount).To(Equal(1), "side-effect node must NOT re-run on resume")
	Expect(c.trigger).To(Equal(1), "trigger must NOT re-run on resume")
	Expect(c.final).To(Equal(1), "final runs once after resume")
}
