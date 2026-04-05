package core

import (
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

func TestConnectionTypeBadString(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeString,
		Value: 1234,
	}

	Expect(c.String()).To(BeNil())
	Expect(c.Number()).To(BeNil())
	Expect(c.Boolean()).To(BeNil())
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
	Expect(ctx.Get("nonexistent")).To(Equal(""))
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
