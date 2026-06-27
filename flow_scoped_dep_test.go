package core

// Regression tests for the scoped-dependency variable substitution
// path (flow.go: `${nodeId.key}` resolution). Two distinct bugs are
// pinned here, both surfaced by execution d402887d-6fc7-4189-8ae6-
// 0d06686d69d1 in production:
//
//   1. Non-firing triggers must NOT be executed as scoped dependencies.
//      Triggers are entry points — only the firing trigger should run
//      in any given execution. Previously, the substitution path
//      called ExecuteNode on ANY referenced node that hadn't run yet,
//      including triggers, causing a Plan Task Trigger to render as
//      "fired" in the execution viewer when the actual trigger was
//      Telegram (and producing empty outputs that leaked the trigger
//      node ID downstream).
//
//   2. Unresolved scoped references must replace as empty string, not
//      leave the literal `${nodeId.key}` text in place. The literal
//      text leaked node IDs into downstream AI prompts; Anthropic
//      models stripped the hyphens and constructed fake blob handles
//      from them (`flo:blob:<uuid-without-hyphens>?size=…&type=…`)
//      that then failed at the action's base64 decoder.

import (
	"testing"

	. "github.com/onsi/gomega"
)

// TestScopedDependency_DoesNotExecuteTriggerNodes locks fix #1.
// A flow has two triggers — telegram (firing) and plan_task (not
// firing). An action wired downstream of telegram references
// `${plan_task.prompt}`. The substitution must NOT call ExecuteNode
// on the plan_task trigger; we verify by counting invocations of
// that action.
func TestScopedDependency_DoesNotExecuteTriggerNodes(t *testing.T) {
	RegisterTestingT(t)

	planTaskInvocations := 0
	triggerInvocations := 0

	telegramAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		triggerInvocations++
		return map[string]interface{}{"chat_id": "12345"}, nil
	}
	planTaskAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		planTaskInvocations++
		return map[string]interface{}{"prompt": "PLAN_TASK_PROMPT_VALUE"}, nil
	}
	consumerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "telegram-trigger", Type: "trigger/telegram", Data: &NodeData{
				Label: "trigger/telegram", Config: NodeConfig{Type: ActionTypeTrigger},
			}},
			{ID: "plan-task-trigger", Type: "trigger/plan_task", Data: &NodeData{
				Label: "trigger/plan_task", Config: NodeConfig{Type: ActionTypeTrigger},
			}},
			{ID: "consumer", Type: "test/consumer", Data: &NodeData{
				Label: "test/consumer",
				Config: NodeConfig{
					Type: ActionTypeAction,
					Inputs: []*Connection{
						// Mimics the failing execution's input wiring:
						// the consumer references the plan_task trigger's
						// `prompt` output, but only telegram fires.
						{Name: "prompt_field", Type: ConnectionTypeString, Value: "${plan-task-trigger.prompt} Try again"},
					},
				},
			}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "telegram-trigger", Target: "consumer"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/telegram":  telegramAction,
		"trigger/plan_task": planTaskAction,
		"test/consumer":     consumerAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil())

	// Telegram fired (the actual trigger).
	Expect(triggerInvocations).To(Equal(1), "telegram trigger should fire exactly once")

	// Plan Task did NOT fire — the scoped-dependency path must skip
	// trigger nodes.
	Expect(planTaskInvocations).To(Equal(0),
		"plan_task trigger must NOT be executed as a scoped dependency — only the firing trigger should run")
}

// TestScopedDependency_UnresolvedReferenceReplacesWithEmpty locks
// fix #2. When a `${nodeId.key}` reference cannot be resolved (the
// node didn't run, OR the requested key isn't in its results), the
// engine must replace the literal `${...}` with empty string so the
// raw text never reaches downstream actions or AI prompts.
//
// Before this fix the literal stayed in place, which is what let the
// AI extract a UUID from `${a038b648-…-….prompt}` and construct a
// fake blob handle from it in production.
func TestScopedDependency_UnresolvedReferenceReplacesWithEmpty(t *testing.T) {
	RegisterTestingT(t)

	var capturedPrompt string

	triggerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	consumerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		for _, c := range inputs {
			if c.Name == "prompt_field" {
				if s := c.String(); s != nil {
					capturedPrompt = *s
				}
			}
		}
		return map[string]interface{}{}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger-1", Type: "trigger/manual", Data: &NodeData{
				Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger},
			}},
			// The referenced trigger that won't fire.
			{ID: "missing-trigger", Type: "trigger/plan_task", Data: &NodeData{
				Label: "trigger/plan_task", Config: NodeConfig{Type: ActionTypeTrigger},
			}},
			{ID: "consumer", Type: "test/consumer", Data: &NodeData{
				Label: "test/consumer",
				Config: NodeConfig{
					Type: ActionTypeAction,
					Inputs: []*Connection{
						{Name: "prompt_field", Type: ConnectionTypeString, Value: "before ${missing-trigger.prompt} after"},
					},
				},
			}},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "consumer"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	actions := map[string]Action{
		"trigger/manual":    triggerAction,
		"trigger/plan_task": triggerAction,
		"test/consumer":     consumerAction,
	}

	_, err := f.Execute(actions, nil, nil)
	Expect(err).To(BeNil())

	// Before this fix, capturedPrompt would have been the literal
	// "before ${missing-trigger.prompt} after". Now the unresolved
	// reference collapses to empty, leaving the surrounding text.
	Expect(capturedPrompt).To(Equal("before  after"),
		"unresolved scoped reference must replace as empty so the literal node ID never reaches downstream actions or AI prompts")
	Expect(capturedPrompt).NotTo(ContainSubstring("${"),
		"no ${...} literal should leak through — that's what lets AI hallucinate fake handles from node UUIDs")
}
