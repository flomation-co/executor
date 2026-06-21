package core

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
)

// --- On Error handled executions should still report as failed ---

func TestOnErrorHandled_StillMarkedAsErrored(t *testing.T) {
	RegisterTestingT(t)

	successAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"ok": true}, nil
	}
	failAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return nil, errors.New("node exploded")
	}
	onErrorAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	recoveryAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"recovered": true}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger-1", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "fail-1", Type: "test/fail", Data: &NodeData{Label: "test/fail", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "on-error-1", Type: "error/on_error", Data: &NodeData{Label: "error/on_error", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "recovery-1", Type: "test/recovery", Data: &NodeData{Label: "test/recovery", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "fail-1"},
			{ID: "e2", Source: "on-error-1", Target: "recovery-1"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": successAction,
		"test/fail":      failAction,
		"error/on_error": onErrorAction,
		"test/recovery":  recoveryAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil()) // Error was handled by On Error chain

	// Even though the On Error chain completed successfully, the flow
	// should still report that an error occurred.
	Expect(f.HadError()).To(BeTrue())
}

func TestNoError_HadErrorIsFalse(t *testing.T) {
	RegisterTestingT(t)

	successAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"ok": true}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger-1", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "action-1", Type: "test/success", Data: &NodeData{Label: "test/success", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "action-1"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": successAction,
		"test/success":   successAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil())
	Expect(f.HadError()).To(BeFalse())
}

// --- On Error chain variable substitution with reachability ---

func TestOnErrorChain_ErrorMessageResolvesWithReachability(t *testing.T) {
	RegisterTestingT(t)

	// When a node fails and the On Error chain fires, children of the
	// On Error node should be able to resolve ${error_message} even
	// though On Error nodes are not forward-reachable from the trigger.

	var capturedErrorMsg string
	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	failAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return nil, errors.New("database connection failed")
	}
	onErrorAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	logErrorAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		for _, c := range inputs {
			if c.Name == "message" && c.String() != nil {
				capturedErrorMsg = *c.String()
			}
		}
		return map[string]interface{}{"logged": true}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger-1", Type: "trigger/manual", Data: &NodeData{Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "fail-1", Type: "test/fail", Data: &NodeData{Label: "test/fail", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "on-error-1", Type: "error/on_error", Data: &NodeData{Label: "error/on_error", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "log-error-1", Type: "test/log_error", Data: &NodeData{
				Label: "test/log_error",
				Config: NodeConfig{
					Type: ActionTypeAction,
					Inputs: []*Connection{
						{Name: "message", Type: ConnectionTypeString, Value: "Error: ${error_message}"},
					},
				},
			}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "fail-1"},
			{ID: "e2", Source: "on-error-1", Target: "log-error-1"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual": triggerAction,
		"test/fail":      failAction,
		"error/on_error": onErrorAction,
		"test/log_error": logErrorAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil())
	Expect(f.HadError()).To(BeTrue())

	// The ${error_message} variable should have been resolved
	Expect(capturedErrorMsg).To(Equal("Error: database connection failed"))
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
		"test/fail":      failAction,
		"error/on_error": onErrorAction,
		"test/child":     childAction,
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

// --- Bug: Parents from unrelated trigger paths should not be executed ---

func TestMultipleTriggerPaths_SkipsUnreachableParent(t *testing.T) {
	RegisterTestingT(t)

	// Scenario: Two triggers feed into the same AI Prompt node.
	// Slack Trigger → AI Prompt (direct)
	// Email Trigger → Switch → AI Prompt
	// When Slack triggers, the Switch (only reachable via Email) must NOT execute.

	var switchExecuted bool
	switchAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		switchExecuted = true
		return map[string]interface{}{"matched_case": "case_0", "result": "switched"}, nil
	}

	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"channel": "slack"}, nil
	}

	promptAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"response": "hello"}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "slack-trigger", Type: "trigger/slack", Data: &NodeData{Label: "trigger/slack", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "email-trigger", Type: "trigger/email", Data: &NodeData{Label: "trigger/email", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "switch-1", Type: "conditional/switch", Data: &NodeData{Label: "conditional/switch", Config: NodeConfig{Type: ActionTypeSwitch}}},
			{ID: "prompt-1", Type: "ai/anthropic", Data: &NodeData{Label: "ai/anthropic", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "slack-trigger", Target: "prompt-1"},
			{ID: "e2", Source: "email-trigger", Target: "switch-1"},
			{ID: "e3", Source: "switch-1", Target: "prompt-1", SourceHandle: "case_0"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	// Execute from the Slack trigger entry point
	slackEntry := "slack-trigger"
	actions := map[string]Action{
		"trigger/slack":      triggerAction,
		"trigger/email":      triggerAction,
		"conditional/switch": switchAction,
		"ai/anthropic":       promptAction,
	}

	_, err := f.Execute(actions, &slackEntry, nil)
	Expect(err).To(BeNil())

	// The Switch node should NOT have been executed — it's only
	// reachable from the Email trigger, not the Slack trigger.
	Expect(switchExecuted).To(BeFalse())

	// The AI Prompt should have executed successfully
	_, promptExecuted := f.nodeResults["prompt-1"]
	Expect(promptExecuted).To(BeTrue())
}

func TestMultipleTriggerPaths_ExecutesReachableParent(t *testing.T) {
	RegisterTestingT(t)

	// Verify that parents reachable from the active trigger ARE still executed.
	// Email Trigger → Switch → AI Prompt

	var switchExecuted bool
	switchAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		switchExecuted = true
		return map[string]interface{}{"matched_case": "case_0"}, nil
	}

	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"channel": "email"}, nil
	}

	promptAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"response": "hello"}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "slack-trigger", Type: "trigger/slack", Data: &NodeData{Label: "trigger/slack", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "email-trigger", Type: "trigger/email", Data: &NodeData{Label: "trigger/email", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "switch-1", Type: "conditional/switch", Data: &NodeData{Label: "conditional/switch", Config: NodeConfig{Type: ActionTypeSwitch}}},
			{ID: "prompt-1", Type: "ai/anthropic", Data: &NodeData{Label: "ai/anthropic", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "slack-trigger", Target: "prompt-1"},
			{ID: "e2", Source: "email-trigger", Target: "switch-1"},
			{ID: "e3", Source: "switch-1", Target: "prompt-1", SourceHandle: "case_0"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	// Execute from the Email trigger entry point
	emailEntry := "email-trigger"
	actions := map[string]Action{
		"trigger/slack":      triggerAction,
		"trigger/email":      triggerAction,
		"conditional/switch": switchAction,
		"ai/anthropic":       promptAction,
	}

	_, err := f.Execute(actions, &emailEntry, nil)
	Expect(err).To(BeNil())

	// The Switch node SHOULD have been executed — it IS reachable from Email trigger
	Expect(switchExecuted).To(BeTrue())
}

// Test: a node behind a regular action on an unmatched Switch branch should
// NOT execute during parent resolution of the AI node.
//
// Flow: Trigger → Switch(voice/text)
//
//	case_0 → STT → DataRename → AI
//	case_1 ─────────────────────→ AI
//
// When Switch matches case_1 (text), data_rename (on case_0 path via STT)
// should NOT execute. Previously, the ancestor walk only recursed through
// Switch/Conditional/Loop nodes, missing the STT action in between.
func TestUnmatchedBranch_SkipsThroughIntermediateActions(t *testing.T) {
	RegisterTestingT(t)

	dataRenameExecuted := false
	sttExecuted := false

	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"channel_type": "telegram"}, nil
	}
	switchAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"matched": true, "matched_case": "case_1", "value": "telegram"}, nil
	}
	sttAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		sttExecuted = true
		return map[string]interface{}{"text": "transcribed"}, nil
	}
	dataRenameAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		dataRenameExecuted = true
		return map[string]interface{}{"content": "renamed"}, nil
	}
	aiAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"response": "hello"}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger", Type: "trigger/telegram", Data: &NodeData{Label: "trigger/telegram", Config: NodeConfig{Type: ActionTypeTrigger}}},
			{ID: "switch", Type: "conditional/switch", Data: &NodeData{Label: "conditional/switch", Config: NodeConfig{Type: ActionTypeSwitch}}},
			{ID: "stt", Type: "action", Data: &NodeData{Label: "elevenlabs/speech_to_text", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "data-rename", Type: "action", Data: &NodeData{Label: "common/data_rename", Config: NodeConfig{Type: ActionTypeAction}}},
			{ID: "ai", Type: "action", Data: &NodeData{Label: "ai/anthropic", Config: NodeConfig{Type: ActionTypeAction}}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger", Target: "switch"},
			{ID: "e2", Source: "switch", Target: "stt", SourceHandle: "case_0"}, // voice branch
			{ID: "e3", Source: "switch", Target: "ai", SourceHandle: "case_1"},  // text branch
			{ID: "e4", Source: "stt", Target: "data-rename"},
			{ID: "e5", Source: "data-rename", Target: "ai"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/telegram":          triggerAction,
		"conditional/switch":        switchAction,
		"elevenlabs/speech_to_text": sttAction,
		"common/data_rename":        dataRenameAction,
		"ai/anthropic":              aiAction,
	}

	triggerEntry := "trigger"
	_, err := f.Execute(actions, &triggerEntry, nil)
	Expect(err).To(BeNil())

	// data_rename and STT should NOT have executed — they're on the unmatched voice branch
	Expect(dataRenameExecuted).To(BeFalse(), "data_rename on unmatched case_0 branch should not execute")
	Expect(sttExecuted).To(BeFalse(), "STT on unmatched case_0 branch should not execute")
}
