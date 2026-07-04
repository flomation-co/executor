package core

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
)

// awaitCounters tracks which branch nodes fired so tests can assert that
// exactly the chosen option's branch runs on resume.
type awaitCounters struct {
	trigger     int
	await       int
	optionYes   int
	optionNo    int
	timeout     int
	sentMessage int
	injected    interface{}
	resumed     bool
}

// buildAwaitGraph builds trigger → await(Type 7) with three handles:
// option_yes → yesNode, option_no → noNode, timeout → timeoutNode. The stub
// await action mimics the real one: it suspends on the first pass and, on
// resume, reads the injected choice from ResumeData and emits matched_case.
func buildAwaitGraph(c *awaitCounters) (*Flow, map[string]Action) {
	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "await", Type: "humanintheloop/await", Data: &NodeData{Label: "humanintheloop/await", Config: NodeConfig{Type: ActionTypeAwait}}},
			{ID: "yesNode", Type: "test/yes", Data: &NodeData{Label: "test/yes", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "noNode", Type: "test/no", Data: &NodeData{Label: "test/no", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "timeoutNode", Type: "test/timeout", Data: &NodeData{Label: "test/timeout", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "await"},
			{ID: "e2", Source: "await", Target: "yesNode", SourceHandle: "option_yes"},
			{ID: "e3", Source: "await", Target: "noNode", SourceHandle: "option_no"},
			{ID: "e4", Source: "await", Target: "timeoutNode", SourceHandle: "timeout"},
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
		"humanintheloop/await": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			c.await++
			if flow.IsResumedNode(node.ID) {
				c.resumed = true
				rd := flow.GetResumeData()
				a, _ := rd["await"].(map[string]interface{})
				outcome, _ := a["outcome"].(string)
				if outcome == "timeout" {
					return map[string]interface{}{"matched_case": "timeout", "outcome": "timeout"}, nil
				}
				val, _ := a["option_value"].(string)
				return map[string]interface{}{"matched_case": "option_" + val, "outcome": "answered"}, nil
			}
			flow.Suspend(&SuspendInfo{
				NodeID:             node.ID,
				Reason:             "await_human",
				ResumeTriggerType:  "hitl_response",
				ResumeTriggerMatch: map[string]interface{}{"request_id": "req-1"},
			})
			return map[string]interface{}{"matched_case": ""}, ErrSuspended
		},
		"test/yes":     func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) { c.optionYes++; return map[string]interface{}{}, nil },
		"test/no":      func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) { c.optionNo++; return map[string]interface{}{}, nil },
		"test/timeout": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) { c.timeout++; return map[string]interface{}{}, nil },
	}
	return f, actions
}

// resumeWith restores a fresh flow from f1's checkpoint with the given await
// resume payload injected (mimicking the API patching the checkpoint JSONB).
func resumeWith(t *testing.T, c *awaitCounters, f1 *Flow, actions map[string]Action, payload map[string]interface{}) {
	t.Helper()
	cp := f1.CreateCheckpoint()
	cp.ResumeData = map[string]interface{}{"await": payload}
	cp = roundTripCheckpoint(t, cp)

	f2, _ := buildAwaitGraph(c)
	f2.RestoreCheckpoint(cp)
	Expect(f2.IsResumedNode("await")).To(BeTrue())
	_, err := f2.Execute(actions, nil, nil)
	Expect(err).To(BeNil(), "resume should complete cleanly")
}

func TestAwait_FirstPassSuspends(t *testing.T) {
	RegisterTestingT(t)
	c := &awaitCounters{}
	f1, actions := buildAwaitGraph(c)

	_, err := f1.Execute(actions, nil, nil)
	Expect(errors.Is(err, ErrSuspended)).To(BeTrue(), "first pass must suspend")
	Expect(c.await).To(Equal(1))
	Expect(c.optionYes + c.optionNo + c.timeout).To(Equal(0), "no branch runs before a response")

	cp := f1.CreateCheckpoint()
	Expect(cp.SuspendInfo).NotTo(BeNil())
	Expect(cp.SuspendInfo.ResumeTriggerType).To(Equal("hitl_response"))
	Expect(cp.SuspendInfo.ResumeTriggerMatch["request_id"]).To(Equal("req-1"))
}

func TestAwait_ResumeOption_RoutesToChosenHandle(t *testing.T) {
	RegisterTestingT(t)
	c := &awaitCounters{}
	f1, actions := buildAwaitGraph(c)
	_, err := f1.Execute(actions, nil, nil)
	Expect(errors.Is(err, ErrSuspended)).To(BeTrue())

	resumeWith(t, c, f1, actions, map[string]interface{}{"outcome": "option", "option_value": "yes", "answered_by": "U123"})

	Expect(c.resumed).To(BeTrue())
	Expect(c.optionYes).To(Equal(1), "chosen option branch runs")
	Expect(c.optionNo).To(Equal(0), "unchosen option branch must be pruned")
	Expect(c.timeout).To(Equal(0), "timeout branch must be pruned")
	Expect(c.trigger).To(Equal(1), "upstream must not re-run on resume")
}

func TestAwait_ResumeTimeout_RoutesToTimeoutHandle(t *testing.T) {
	RegisterTestingT(t)
	c := &awaitCounters{}
	f1, actions := buildAwaitGraph(c)
	_, err := f1.Execute(actions, nil, nil)
	Expect(errors.Is(err, ErrSuspended)).To(BeTrue())

	resumeWith(t, c, f1, actions, map[string]interface{}{"outcome": "timeout"})

	Expect(c.timeout).To(Equal(1), "timeout branch runs")
	Expect(c.optionYes).To(Equal(0))
	Expect(c.optionNo).To(Equal(0))
}

// TestAwait_DeliversViaHandle_NoRecursion validates the AI-tools-style wiring:
// a Send node connected to the Await node's "delivery" handle is invoked
// in-process during the first pass. Because the Send node's parent IS the
// (mid-execution) Await node, this exercises InvokeNode's recursion guard.
func TestAwait_DeliversViaHandle_NoRecursion(t *testing.T) {
	RegisterTestingT(t)
	var sendRuns int
	var sentMessage interface{}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "await", Type: "humanintheloop/await", Data: &NodeData{Label: "humanintheloop/await", Config: NodeConfig{Type: ActionTypeAwait}}},
			{ID: "send", Type: "slack/send_message", Data: &NodeData{Label: "slack/send_message", Config: NodeConfig{Type: ActionTypeAction,
				Inputs: []*Connection{{Name: "message", Type: ConnectionTypeText, Value: "original"}}}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "await"},
			{ID: "e2", Source: "await", Target: "send", SourceHandle: DeliveryHandle},
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
		"humanintheloop/await": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			if flow.IsResumedNode(node.ID) {
				return map[string]interface{}{"matched_case": "option_yes"}, nil
			}
			// Deliver via the delivery handle, in-process (like the real action).
			for _, target := range flow.FindTargetByHandle(node.ID, DeliveryHandle) {
				if _, err := flow.InvokeNode(target, map[string]interface{}{"message": "please approve"}); err != nil {
					return nil, err
				}
			}
			flow.Suspend(&SuspendInfo{NodeID: node.ID, Reason: "await_human"})
			return map[string]interface{}{"matched_case": ""}, ErrSuspended
		},
		"slack/send_message": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			sendRuns++
			if m := FindConnection("message", inputs); m != nil && m.String() != nil {
				sentMessage = *m.String()
			}
			return map[string]interface{}{"success": true}, nil
		},
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(errors.Is(err, ErrSuspended)).To(BeTrue(), "await must suspend after delivering")
	Expect(sendRuns).To(Equal(1), "delivery node runs exactly once — no recursion")
	Expect(sentMessage).To(Equal("please approve"), "injected message reaches the delivery node")
}

// TestInvokeNode_DeliversToReferencedNode validates the in-process fan-out
// mechanism: an action calls flow.InvokeNode on an edge-less "send" node,
// injecting a value, and the send node runs with that value and never runs
// on its own during normal traversal.
func TestInvokeNode_DeliversToReferencedNode(t *testing.T) {
	RegisterTestingT(t)
	c := &awaitCounters{}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "caller", Type: "test/caller", Data: &NodeData{Label: "test/caller", Config: NodeConfig{Type: ActionTypeAction}}},
			// send is edge-less: never reached by normal traversal, only via InvokeNode.
			{ID: "send", Type: "test/send", Data: &NodeData{Label: "test/send", Config: NodeConfig{Type: ActionTypeAction,
				Inputs: []*Connection{{Name: "message", Type: ConnectionTypeText, Value: "original"}}}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "caller"},
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
		"test/caller": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			target := flow.FindNode("send")
			_, err := flow.InvokeNode(target, map[string]interface{}{"message": "injected", "blocks": "[]"})
			return map[string]interface{}{}, err
		},
		"test/send": func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
			c.sentMessage++
			if m := FindConnection("message", inputs); m != nil && m.String() != nil {
				c.injected = *m.String()
			}
			return map[string]interface{}{"ok": true}, nil
		},
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil())
	Expect(c.sentMessage).To(Equal(1), "send node runs exactly once, via InvokeNode")
	Expect(c.injected).To(Equal("injected"), "injected value overrides the node's configured input")

	// The transient injected 'blocks' input must have been stripped, and the
	// 'message' input restored to its original saved value.
	send := f.FindNode("send")
	var names []string
	var msgVal interface{}
	for _, in := range send.Data.Config.Inputs {
		names = append(names, in.Name)
		if in.Name == "message" {
			msgVal = in.Value
		}
	}
	Expect(names).NotTo(ContainElement("blocks"), "transient injected input must be stripped after delivery")
	Expect(msgVal).To(Equal("original"), "declared input must be restored to its saved value")
}
