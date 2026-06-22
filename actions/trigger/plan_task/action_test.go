package plan_task

// Tests for the Plan Task Trigger node. The trigger has no behavioural
// logic of its own — the tick endpoint (M1.5 commit 3) populates
// trigger data and the executor projects it through. What we DO pin
// here is the OUTPUT SHAPE: every name + type the API's commit 3
// dispatch relies on. A drift in either side breaks the contract
// silently — variable substitution would resolve to undefined.
//
// The "channel-shape mirror" outputs (prompt, channel_type,
// conversation_history, etc.) are load-bearing because the agent's
// flow author wires the AI Prompt action against ${flow.prompt} once
// and expects it to work for both channel triggers and plan tasks.

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// outputByName returns the Connection in Outputs with the given name,
// or nil if absent. Lets each assertion read its own expected slot.
func outputByName(name string) *core.Connection {
	for i := range Outputs {
		if Outputs[i].Name == name {
			return &Outputs[i]
		}
	}
	return nil
}

func TestPlanTaskTrigger_Metadata(t *testing.T) {
	RegisterTestingT(t)
	Expect(Name).To(Equal("Plan Task Trigger"))
	Expect(Type).To(Equal(core.ActionTypeTrigger))
	Expect(Icon).NotTo(BeEmpty())
	Expect(Description).NotTo(BeEmpty())
}

// TestPlanTaskTrigger_ChannelShapedOutputsPresent guards the
// "no flow amendment" promise. The author wires the AI Prompt action
// against ${flow.prompt} and ${flow.conversation_history}; those
// outputs MUST exist and carry the right types or the wiring breaks
// silently when a plan task fires.
func TestPlanTaskTrigger_ChannelShapedOutputsPresent(t *testing.T) {
	RegisterTestingT(t)
	expectations := map[string]string{
		"prompt":               core.ConnectionTypeText,
		"channel_type":         core.ConnectionTypeString,
		"channel_id":           core.ConnectionTypeString,
		"conversation_history": core.ConnectionTypeObject,
		"agent_id":             core.ConnectionTypeString,
		"agent_user_id":        core.ConnectionTypeString,
	}
	for name, wantType := range expectations {
		out := outputByName(name)
		Expect(out).NotTo(BeNil(), "channel-shaped output %q missing", name)
		Expect(out.Type).To(Equal(wantType), "channel-shaped output %q has wrong type", name)
	}
}

// TestPlanTaskTrigger_PlanSpecificOutputsPresent pins the new fields
// the agent's tools (set_output, plan/block in M1.5 commit 6) read.
func TestPlanTaskTrigger_PlanSpecificOutputsPresent(t *testing.T) {
	RegisterTestingT(t)
	expectations := map[string]string{
		"plan_id":               core.ConnectionTypeString,
		"plan_task_id":          core.ConnectionTypeString,
		"plan_task_name":        core.ConnectionTypeString,
		"plan_task_description": core.ConnectionTypeText,
		"plan_task_inputs":      core.ConnectionTypeObject,
		"upstream_outputs":      core.ConnectionTypeObject,
	}
	for name, wantType := range expectations {
		out := outputByName(name)
		Expect(out).NotTo(BeNil(), "plan-specific output %q missing", name)
		Expect(out.Type).To(Equal(wantType), "plan-specific output %q has wrong type", name)
	}
}

// TestExecute_ReflectsInputsToOutputs — the engine populates inputs
// from trigger data (set by the tick endpoint) and the Execute func
// just reflects them as outputs so downstream ${flow.X} substitution
// resolves. We assert the reflection works for both string and
// object-shaped values.
func TestExecute_ReflectsInputsToOutputs(t *testing.T) {
	RegisterTestingT(t)
	strPtr := func(s string) *core.Connection {
		return &core.Connection{Name: "channel_type", Type: core.ConnectionTypeString, Value: s}
	}
	objMap := map[string]interface{}{"foo": "bar"}
	inputs := []*core.Connection{
		strPtr("plan_task"),
		{Name: "plan_id", Type: core.ConnectionTypeString, Value: "plan-1"},
		{Name: "upstream_outputs", Type: core.ConnectionTypeObject, Value: objMap},
		{Name: "empty_value", Type: core.ConnectionTypeString, Value: nil},
		nil, // defensive: tick may pass nils for absent fields
	}
	got, err := Execute(nil, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(HaveKeyWithValue("channel_type", "plan_task"))
	Expect(got).To(HaveKeyWithValue("plan_id", "plan-1"))
	Expect(got).To(HaveKeyWithValue("upstream_outputs", objMap))
	// nil-valued inputs are skipped — engine produces them for fields
	// the tick didn't populate; downstream substitution treats missing
	// keys as empty.
	Expect(got).NotTo(HaveKey("empty_value"))
}
