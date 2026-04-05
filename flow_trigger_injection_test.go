package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

// TestInjectTriggerDataSkipsReservedKeys ensures that agent-orchestration
// keys users compose into text (system_prompt) are NOT injected as inputs
// on the trigger node. Regression: trigger actions typically re-emit their
// inputs as outputs, which would auto-wire into same-named inputs on
// downstream actions (e.g. an AI node's system_prompt), silently overriding
// user-authored composed text like "${flow.system_prompt}\n\n<directives>".
//
// Non-composed pass-through values like conversation_history must continue
// to flow through the trigger-echo relay so that downstream AI actions can
// pick them up via auto-wire or via an explicit ${conversation_history}
// whole-value reference.
func TestInjectTriggerDataSkipsReservedKeys(t *testing.T) {
	RegisterTestingT(t)

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/slack",
				Data: &NodeData{
					Label: "trigger/slack",
					Config: NodeConfig{
						ID:     "trigger-1",
						Type:   ActionTypeTrigger,
						Inputs: []*Connection{},
					},
				},
			},
		},
	}

	convHistory := []interface{}{map[string]interface{}{"role": "user", "content": "hi"}}
	data := map[string]interface{}{
		"channel_type":         "slack",
		"system_prompt":        "You are a helpful agent.",
		"conversation_history": convHistory,
		"content":              "Hello there",
		"sender":               "alice",
	}

	f.InjectTriggerData(data)

	inputs := f.Nodes[0].Data.Config.Inputs
	names := map[string]interface{}{}
	for _, in := range inputs {
		names[in.Name] = in.Value
	}

	// Reserved (composed-text) keys must NOT be injected.
	_, hasSystemPrompt := names["system_prompt"]
	Expect(hasSystemPrompt).To(BeFalse(), "system_prompt must not be injected onto the trigger node")

	// Pass-through keys including conversation_history MUST still be
	// injected so they flow through the trigger's output relay.
	Expect(names["conversation_history"]).To(Equal(convHistory))
	Expect(names["content"]).To(Equal("Hello there"))
	Expect(names["sender"]).To(Equal("alice"))
	Expect(names["channel_type"]).To(Equal("slack"))
}

// TestExplicitInputOverridesAutoWiredParentOutput verifies that when a node
// has an explicitly set input, it is NOT overwritten by a parent's output
// that happens to share the same name. Regression: the bug that inspired
// this test was an AI action's system_prompt input being clobbered by the
// Slack trigger re-emitting an injected system_prompt as one of its outputs.
func TestExplicitInputOverridesAutoWiredParentOutput(t *testing.T) {
	RegisterTestingT(t)

	// Trigger echoes all its inputs (including injected trigger-data keys)
	// back out as outputs, matching the real Slack/Telegram/Webhook pattern.
	echoInputs := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		result := make(map[string]interface{})
		for _, c := range inputs {
			if c.Value != nil {
				result[c.Name] = c.Value
			}
		}
		return result, nil
	}

	// Downstream action captures its resolved inputs so the test can assert
	// on what the action actually received.
	var receivedSystemPrompt string
	captureAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		for _, c := range inputs {
			if c.Name == "system_prompt" && c.String() != nil {
				receivedSystemPrompt = *c.String()
			}
		}
		return map[string]interface{}{"ok": true}, nil
	}

	actions := map[string]Action{
		"trigger/echo": echoInputs,
		"action/ai":    captureAction,
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/echo",
				Data: &NodeData{
					Label: "trigger/echo",
					Config: NodeConfig{
						ID:   "trigger-1",
						Type: ActionTypeTrigger,
						// Simulate a parent that emits system_prompt as an output.
						Inputs: []*Connection{
							{Name: "system_prompt", Type: ConnectionTypeString, Value: "BARE AGENT PROMPT"},
						},
					},
				},
			},
			{
				ID:   "action-1",
				Type: "action/ai",
				Data: &NodeData{
					Label: "action/ai",
					Config: NodeConfig{
						ID:   "action-1",
						Type: ActionTypeAction,
						Inputs: []*Connection{
							{
								Name:  "system_prompt",
								Type:  ConnectionTypeText,
								Value: "${flow.system_prompt}\n\n### MARKER-v1 ###\nRespond using ${trigger.channel_type} formatting.",
							},
						},
					},
				},
			},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "action-1"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	ctx := &ExecutionContext{
		FlowID:       "flo-abc",
		ExecutionID:  "exec-xyz",
		SystemPrompt: "You are a helpful agent.",
	}
	f.SetContext(ctx)

	entry := "trigger-1"
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())

	// The AI action must see the agent's system prompt followed by the
	// user-authored marker and directives. The parent's same-named output
	// must NOT have clobbered the explicit input.
	Expect(receivedSystemPrompt).To(ContainSubstring("You are a helpful agent."))
	Expect(receivedSystemPrompt).To(ContainSubstring("### MARKER-v1 ###"))
	Expect(receivedSystemPrompt).NotTo(Equal("BARE AGENT PROMPT"))
}

// TestConversationHistoryFlowsThroughTriggerRelay verifies that non-reserved
// trigger-data keys — specifically conversation_history, which is the array
// value that powers multi-turn agent conversations — are still injected
// onto the trigger node, re-emitted as outputs by the trigger action, and
// auto-wired into a downstream AI action's same-named input. Regression
// guard: an earlier iteration of the reserved-key filter included
// conversation_history, which broke end-to-end message history delivery.
func TestConversationHistoryFlowsThroughTriggerRelay(t *testing.T) {
	RegisterTestingT(t)

	// Trigger echoes all its inputs (including injected trigger-data keys)
	// as outputs, matching the real Slack/Telegram/Webhook action pattern.
	echoInputs := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		result := make(map[string]interface{})
		for _, c := range inputs {
			if c.Value != nil {
				result[c.Name] = c.Value
			}
		}
		return result, nil
	}

	var receivedHistory interface{}
	captureAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		for _, c := range inputs {
			if c.Name == "conversation_history" {
				receivedHistory = c.Value
			}
		}
		return map[string]interface{}{"ok": true}, nil
	}

	actions := map[string]Action{
		"trigger/slack": echoInputs,
		"action/ai":     captureAction,
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/slack",
				Data: &NodeData{
					Label:  "trigger/slack",
					Config: NodeConfig{ID: "trigger-1", Type: ActionTypeTrigger},
				},
			},
			{
				ID:   "action-1",
				Type: "action/ai",
				Data: &NodeData{
					Label: "action/ai",
					Config: NodeConfig{
						ID:   "action-1",
						Type: ActionTypeAction,
						Inputs: []*Connection{
							// Blank input — should auto-wire from the trigger's
							// echoed conversation_history output.
							{Name: "conversation_history", Type: ConnectionTypeObject, Value: nil},
						},
					},
				},
			},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "action-1"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	convHistory := []interface{}{
		map[string]interface{}{"role": "user", "content": "hi"},
		map[string]interface{}{"role": "assistant", "content": "hello!"},
	}
	f.InjectTriggerData(map[string]interface{}{
		"channel_type":         "slack",
		"conversation_history": convHistory,
		"content":              "how are you?",
	})

	entry := "trigger-1"
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())
	Expect(receivedHistory).To(Equal(convHistory))
}

// TestBlankInputFallsBackToParentOutput confirms that the "explicit wins"
// rule does NOT break the existing convenience where an unset input on a
// downstream node is automatically wired from a matching-named parent
// output. Only explicitly set values should take precedence.
func TestBlankInputFallsBackToParentOutput(t *testing.T) {
	RegisterTestingT(t)

	emitContent := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"content": "hello from parent"}, nil
	}

	var receivedContent interface{}
	captureAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		for _, c := range inputs {
			if c.Name == "content" {
				receivedContent = c.Value
			}
		}
		return map[string]interface{}{"ok": true}, nil
	}

	actions := map[string]Action{
		"trigger/emit": emitContent,
		"action/sink":  captureAction,
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/emit",
				Data: &NodeData{
					Label:  "trigger/emit",
					Config: NodeConfig{ID: "trigger-1", Type: ActionTypeTrigger},
				},
			},
			{
				ID:   "action-1",
				Type: "action/sink",
				Data: &NodeData{
					Label: "action/sink",
					Config: NodeConfig{
						ID:   "action-1",
						Type: ActionTypeAction,
						Inputs: []*Connection{
							// Blank (empty string) — should fall back to parent.
							{Name: "content", Type: ConnectionTypeString, Value: ""},
						},
					},
				},
			},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "action-1"},
		},
		nodeResults: make(map[string]map[string]interface{}),
		outputs:     make(map[string]interface{}),
	}

	entry := "trigger-1"
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())
	Expect(receivedContent).To(Equal("hello from parent"))
}
