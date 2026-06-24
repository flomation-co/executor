package core

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
)

func TestConnectionTypeString(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeString,
		Value: "abcdef12345",
	}

	Expect(c.String()).To(Not(BeNil()))

	Expect(c.Number()).To(BeNil())
	Expect(c.Boolean()).To(BeNil())
}

// When a non-string value lands in a String-typed Connection (e.g. an
// upstream output that is a number, slice, or map flowing into a string
// input via ${parent.X}), String() now JSON-encodes the value rather
// than returning nil. Returning nil here would silently break wired-up
// flows — the action would see an empty input and skip its job — so
// the fallback explicitly favours visibility. See flow.go:248-253.
func TestConnectionTypeString_NonStringValueJSONEncoded(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeString,
		Value: 1234,
	}

	s := c.String()
	Expect(s).NotTo(BeNil())
	Expect(*s).To(Equal("1234"))

	// Number/Boolean still return nil — the JSON-encoded fallback is
	// only on the String path.
	Expect(c.Number()).To(BeNil())
	Expect(c.Boolean()).To(BeNil())
}

// Companion case: a value that genuinely cannot be JSON-marshalled
// (channels, funcs) returns nil from String(). This locks in the
// "last resort" nil branch at flow.go:262.
func TestConnectionTypeString_UnmarshalableValueReturnsNil(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeString,
		Value: make(chan int),
	}

	Expect(c.String()).To(BeNil())
}

func TestConnectionTypeNumber(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeInteger,
		Value: 1234,
	}

	Expect(c.Number()).To(Not(BeNil()))

	Expect(*c.String()).To(Equal("1234"))
	Expect(c.Boolean()).To(BeNil())
}

func TestConnectionTypeBadNumber(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeInteger,
		Value: "1234",
	}

	Expect(c.Number()).To(Not(BeNil()))

	Expect(*c.String()).To(Equal("1234"))
	Expect(c.Boolean()).To(BeNil())

	c2 := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeInteger,
		Value: "abcd",
	}

	Expect(c2.Number()).To(BeNil())
	Expect(c2.String()).To(BeNil())
	Expect(c2.Boolean()).To(BeNil())

	c3 := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeInteger,
		Value: 12.3,
	}

	Expect(c3.Number()).To(Not(BeNil()))

	Expect(c3.String()).To(BeNil())
	Expect(c3.Boolean()).To(BeNil())
}

func TestConnectionTypeBoolean(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeBoolean,
		Value: false,
	}

	Expect(c.Boolean()).To(Not(BeNil()))

	Expect(*c.String()).To(Equal("false"))
	Expect(c.Number()).To(BeNil())
}

func TestConnectionTypeBadBoolean(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeBoolean,
		Value: "abc",
	}

	Expect(c.Boolean()).To(BeNil())
	Expect(c.String()).To(BeNil())
	Expect(c.Number()).To(BeNil())
}

func TestFindConnection(t *testing.T) {
	RegisterTestingT(t)

	connections := []*Connection{
		&Connection{
			Name:  "connection1",
			Type:  ConnectionTypeString,
			Value: "value",
		},
	}

	result := FindConnection("connection1", connections)
	Expect(result).To(Not(BeNil()))
	Expect(result).To(Equal(connections[0]))

	bad := FindConnection("missing-connection-name", connections)
	Expect(bad).To(BeNil())
}

func TestExecutionContextGet(t *testing.T) {
	RegisterTestingT(t)

	ctx := ExecutionContext{
		FlowID:         "flo-123",
		ExecutionID:    "exec-456",
		Sequence:       7,
		AuthorID:       "user-001",
		OrganisationID: "org-002",
		RunnerID:       "runner-003",
		StartTime:      "2026-03-23T10:00:00Z",
		TriggerType:    "schedule",
		AuthorEmail:    "author@example.com",
		TriggererEmail: "trigger@example.com",
		SystemPrompt:   "You are helpful.",
		AgentID:        "agent-aaa",
		AgentUserID:    "user-bbb",
		ConversationID: "conv-ccc",
		PlanTaskID:     "task-ddd",
	}

	Expect(ctx.Get("flow_id")).To(Equal("flo-123"))
	Expect(ctx.Get("execution_id")).To(Equal("exec-456"))
	Expect(ctx.Get("sequence")).To(Equal("7"))
	Expect(ctx.Get("author_id")).To(Equal("user-001"))
	Expect(ctx.Get("organisation_id")).To(Equal("org-002"))
	Expect(ctx.Get("runner_id")).To(Equal("runner-003"))
	Expect(ctx.Get("start_time")).To(Equal("2026-03-23T10:00:00Z"))
	Expect(ctx.Get("trigger_type")).To(Equal("schedule"))
	Expect(ctx.Get("author_email")).To(Equal("author@example.com"))
	Expect(ctx.Get("triggerer_email")).To(Equal("trigger@example.com"))
	Expect(ctx.Get("system_prompt")).To(Equal("You are helpful."))
	Expect(ctx.Get("agent_id")).To(Equal("agent-aaa"))
	Expect(ctx.Get("agent_user_id")).To(Equal("user-bbb"))
	Expect(ctx.Get("conversation_id")).To(Equal("conv-ccc"))
	Expect(ctx.Get("plan_task_id")).To(Equal("task-ddd"))
	Expect(ctx.Get("nonexistent")).To(Equal(""))
}

// TestExecutionContextJSONRoundtrip ensures the agent memory fields
// added in Phase 1.5 survive a JSON marshal/unmarshal cycle. This is
// the path the runner actually uses: it marshals a map with these
// keys, writes it to context.json, and the executor reads it back
// into ExecutionContext at flow entry. A regression here would mean
// the executor silently loses agent scoping on every execution.
func TestExecutionContextJSONRoundtrip(t *testing.T) {
	RegisterTestingT(t)

	original := ExecutionContext{
		FlowID:         "flo-1",
		AgentID:        "agent-aaa",
		AgentUserID:    "user-bbb",
		ConversationID: "conv-ccc",
		PlanTaskID:     "task-ddd",
	}

	raw, err := json.Marshal(original)
	Expect(err).To(BeNil())
	// Marshalled output must use the canonical snake_case keys — the
	// runner writes these keys literally into context.json.
	Expect(string(raw)).To(ContainSubstring(`"agent_id":"agent-aaa"`))
	Expect(string(raw)).To(ContainSubstring(`"agent_user_id":"user-bbb"`))
	Expect(string(raw)).To(ContainSubstring(`"conversation_id":"conv-ccc"`))
	Expect(string(raw)).To(ContainSubstring(`"plan_task_id":"task-ddd"`))

	var decoded ExecutionContext
	Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
	Expect(decoded.AgentID).To(Equal("agent-aaa"))
	Expect(decoded.AgentUserID).To(Equal("user-bbb"))
	Expect(decoded.ConversationID).To(Equal("conv-ccc"))
	Expect(decoded.PlanTaskID).To(Equal("task-ddd"))
}

// TestExecutionContextJSONRoundtrip_EmptyFieldsOmitted ensures a
// non-agent execution produces a clean context.json without noisy
// empty agent_* keys. The `omitempty` tags guard this, and the test
// pins the invariant so a future field-add doesn't accidentally drop
// the tag.
func TestExecutionContextJSONRoundtrip_EmptyFieldsOmitted(t *testing.T) {
	RegisterTestingT(t)

	ctx := ExecutionContext{FlowID: "flo-1"}
	raw, err := json.Marshal(ctx)
	Expect(err).To(BeNil())

	Expect(string(raw)).NotTo(ContainSubstring("agent_id"))
	Expect(string(raw)).NotTo(ContainSubstring("agent_user_id"))
	Expect(string(raw)).NotTo(ContainSubstring("conversation_id"))
	Expect(string(raw)).NotTo(ContainSubstring("system_prompt"))
	Expect(string(raw)).NotTo(ContainSubstring("plan_task_id"))
}

func TestFlowVariableSubstitution(t *testing.T) {
	RegisterTestingT(t)

	echoAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		result := make(map[string]interface{})
		for _, c := range inputs {
			if c.String() != nil {
				result[c.Name] = *c.String()
			}
		}
		return result, nil
	}

	actions := map[string]Action{
		"trigger/manual": echoAction,
		"action/echo":    echoAction,
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/manual",
				Data: &NodeData{
					Label: "trigger/manual",
					Config: NodeConfig{
						ID:   "trigger-1",
						Type: ActionTypeTrigger,
					},
				},
			},
			{
				ID:   "action-1",
				Type: "action/echo",
				Data: &NodeData{
					Label: "action/echo",
					Config: NodeConfig{
						ID:   "action-1",
						Type: ActionTypeAction,
						Inputs: []*Connection{
							{Name: "exec_id", Type: ConnectionTypeString, Value: "Execution: ${flow.execution_id}"},
							{Name: "flow_id", Type: ConnectionTypeString, Value: "${flow.flow_id}"},
							{Name: "runner", Type: ConnectionTypeString, Value: "${flow.runner_id}"},
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
		FlowID:      "flo-abc",
		ExecutionID: "exec-xyz",
		RunnerID:    "runner-42",
	}
	f.SetContext(ctx)

	entry := "trigger-1"
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())

	result := f.GetNodeResult("action-1")
	Expect(result).To(Not(BeNil()))
	Expect(result["exec_id"]).To(Equal("Execution: exec-xyz"))
	Expect(result["flow_id"]).To(Equal("flo-abc"))
	Expect(result["runner"]).To(Equal("runner-42"))
}

// TestFlowWholeValueReferencePreservesType verifies that when an input's
// entire value is a single ${name} reference and the referenced parent
// output is a non-string value (e.g. an array of conversation messages),
// the raw typed value is preserved on the downstream input rather than
// being stringified via fmt.Sprintf. This is required for AI actions that
// accept a conversation_history array.
func TestFlowWholeValueReferencePreservesType(t *testing.T) {
	RegisterTestingT(t)

	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		result := make(map[string]interface{})
		for _, input := range inputs {
			if input.Value != nil {
				result[input.Name] = input.Value
			}
		}
		return result, nil
	}

	var captured interface{}
	capturingAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		for _, c := range inputs {
			if c.Name == "history" {
				captured = c.Value
			}
		}
		return map[string]interface{}{}, nil
	}

	actions := map[string]Action{
		"trigger/manual": triggerAction,
		"action/ai":      capturingAction,
	}

	history := []map[string]string{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": "hi there"},
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/manual",
				Data: &NodeData{
					Label: "trigger/manual",
					Config: NodeConfig{
						ID:   "trigger-1",
						Type: ActionTypeTrigger,
						Inputs: []*Connection{
							{Name: "conversation_history", Type: ConnectionTypeObject, Value: history},
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
							{Name: "history", Type: ConnectionTypeObject, Value: "${conversation_history}"},
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

	// captured must be the typed slice, not a stringified form.
	slice, ok := captured.([]map[string]string)
	Expect(ok).To(BeTrue(), "expected raw []map[string]string, got %T", captured)
	Expect(slice).To(HaveLen(2))
	Expect(slice[0]["role"]).To(Equal("user"))
	Expect(slice[1]["content"]).To(Equal("hi there"))
}

func TestFlowVariableSubstitutionWithoutContext(t *testing.T) {
	RegisterTestingT(t)

	echoAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		result := make(map[string]interface{})
		for _, c := range inputs {
			if c.String() != nil {
				result[c.Name] = *c.String()
			}
		}
		return result, nil
	}

	actions := map[string]Action{
		"trigger/manual": echoAction,
		"action/echo":    echoAction,
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/manual",
				Data: &NodeData{
					Label: "trigger/manual",
					Config: NodeConfig{
						ID:   "trigger-1",
						Type: ActionTypeTrigger,
					},
				},
			},
			{
				ID:   "action-1",
				Type: "action/echo",
				Data: &NodeData{
					Label: "action/echo",
					Config: NodeConfig{
						ID:   "action-1",
						Type: ActionTypeAction,
						Inputs: []*Connection{
							{Name: "exec_id", Type: ConnectionTypeString, Value: "${flow.execution_id}"},
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

	// No context set — variable should remain unresolved
	entry := "trigger-1"
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())

	result := f.GetNodeResult("action-1")
	Expect(result).To(Not(BeNil()))
	// Variable stays as-is when no context is available
	Expect(result["exec_id"]).To(Equal("${flow.execution_id}"))
}

// === M3.5 — plan-task tool filtering ===

// TestInjectToolDefinitions_PlanTaskMode_FiltersForbiddenTools pins
// the M3.5 fix: when the flow's ChannelType is "plan_task", the AI
// must not see plan/create or plan/cancel in its tool list. Without
// this filter, an agent running inside a plan task can recursively
// spawn new plans (the runaway-loop we hit in testing).
func TestInjectToolDefinitions_PlanTaskMode_FiltersForbiddenTools(t *testing.T) {
	RegisterTestingT(t)

	aiNode := &Node{
		ID: "ai-1",
		Data: &NodeData{Label: "ai/anthropic", Config: NodeConfig{ID: "ai-1"}},
	}
	toolNodes := []*Node{
		{ID: "t1", Data: &NodeData{Label: "plan/create", Config: NodeConfig{ID: "t1", Inputs: []*Connection{}}}},
		{ID: "t2", Data: &NodeData{Label: "plan/cancel", Config: NodeConfig{ID: "t2", Inputs: []*Connection{}}}},
		{ID: "t3", Data: &NodeData{Label: "plan/get_status", Config: NodeConfig{ID: "t3", Inputs: []*Connection{}}}},
		{ID: "t4", Data: &NodeData{Label: "plan/block", Config: NodeConfig{ID: "t4", Inputs: []*Connection{}}}},
		{ID: "t5", Data: &NodeData{Label: "output/set_output", Config: NodeConfig{ID: "t5", Inputs: []*Connection{}}}},
		// M5: plan/revise is also filtered — a plan task should
		// not mutate its parent plan's task graph.
		{ID: "t6", Data: &NodeData{Label: "plan/revise", Config: NodeConfig{ID: "t6", Inputs: []*Connection{}}}},
	}

	f := &Flow{
		context: &ExecutionContext{ChannelType: "plan_task"},
	}
	f.injectToolDefinitions(aiNode, toolNodes, map[string]Action{})

	// Inspect tool_definitions on aiNode's inputs.
	var defs string
	for _, inp := range aiNode.Data.Config.Inputs {
		if inp.Name == "tool_definitions" && inp.Value != nil {
			if s, ok := inp.Value.(string); ok {
				defs = s
			}
		}
	}
	Expect(defs).NotTo(BeEmpty(), "tool_definitions must be set")

	// Forbidden tools must NOT appear.
	Expect(defs).NotTo(ContainSubstring(`"plan_create"`))
	Expect(defs).NotTo(ContainSubstring(`"plan_cancel"`))
	Expect(defs).NotTo(ContainSubstring(`"plan_revise"`))
	// Allowed tools MUST appear.
	Expect(defs).To(ContainSubstring(`"plan_get_status"`))
	Expect(defs).To(ContainSubstring(`"plan_block"`))
	Expect(defs).To(ContainSubstring(`"output_set_output"`))
}

// TestInjectToolDefinitions_NonPlanTaskMode_KeepsAllTools confirms
// the filter is scoped to plan-task mode only. Telegram, Slack,
// manual triggers etc. must see the full tool list — including
// plan/create so the agent CAN author plans on user-facing turns.
func TestInjectToolDefinitions_NonPlanTaskMode_KeepsAllTools(t *testing.T) {
	RegisterTestingT(t)

	aiNode := &Node{
		ID: "ai-1",
		Data: &NodeData{Label: "ai/anthropic", Config: NodeConfig{ID: "ai-1"}},
	}
	toolNodes := []*Node{
		{ID: "t1", Data: &NodeData{Label: "plan/create", Config: NodeConfig{ID: "t1", Inputs: []*Connection{}}}},
		{ID: "t2", Data: &NodeData{Label: "plan/cancel", Config: NodeConfig{ID: "t2", Inputs: []*Connection{}}}},
	}

	f := &Flow{
		context: &ExecutionContext{ChannelType: "telegram"},
	}
	f.injectToolDefinitions(aiNode, toolNodes, map[string]Action{})

	var defs string
	for _, inp := range aiNode.Data.Config.Inputs {
		if inp.Name == "tool_definitions" && inp.Value != nil {
			if s, ok := inp.Value.(string); ok {
				defs = s
			}
		}
	}
	Expect(defs).To(ContainSubstring(`"plan_create"`))
	Expect(defs).To(ContainSubstring(`"plan_cancel"`))
}

// TestInjectToolDefinitions_NoContext_KeepsAllTools is the
// defensive path — if the flow has no execution context (eg unit
// tests, manual triggers without channel framing) we MUST NOT
// filter, because we don't know what mode we're in. The safe
// default is "show everything".
func TestInjectToolDefinitions_NoContext_KeepsAllTools(t *testing.T) {
	RegisterTestingT(t)

	aiNode := &Node{
		ID: "ai-1",
		Data: &NodeData{Label: "ai/anthropic", Config: NodeConfig{ID: "ai-1"}},
	}
	toolNodes := []*Node{
		{ID: "t1", Data: &NodeData{Label: "plan/create", Config: NodeConfig{ID: "t1", Inputs: []*Connection{}}}},
	}

	f := &Flow{
		context: nil, // explicit nil — defensive path
	}
	f.injectToolDefinitions(aiNode, toolNodes, map[string]Action{})

	var defs string
	for _, inp := range aiNode.Data.Config.Inputs {
		if inp.Name == "tool_definitions" && inp.Value != nil {
			if s, ok := inp.Value.(string); ok {
				defs = s
			}
		}
	}
	Expect(defs).To(ContainSubstring(`"plan_create"`))
}
