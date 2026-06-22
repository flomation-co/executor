// Package plan_task — the Plan Task Trigger node. Drops into the
// canvas alongside Telegram / Slack / Manual triggers and gives the
// agent's orchestrator flow an entry point for plan-task invocations.
//
// Fired INTERNALLY by the API's tick endpoint (commit 3 of M1.5) when
// an orchestrator-kind plan task becomes ready. The tick populates
// trigger data with task framing — `prompt`, `channel_type='plan_task'`,
// plan_id, plan_task_id, plan_task_name, plan_task_description,
// plan_task_inputs, upstream_outputs, empty conversation_history.
//
// Output names mirror what user-message triggers (Telegram, Slack,
// Manual, etc.) populate so the downstream AI Prompt action reads
// `${flow.prompt}` and `${flow.conversation_history}` identically
// regardless of which trigger fired. The agent's existing flow needs
// ZERO rewiring — drop this node alongside the channel triggers and
// route it into the same AI Prompt action.
//
// See plans/agent_planning_m1_5.md commit 2 for the design.
package plan_task

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Plan Task Trigger"
	Description  = "Fires when a plan task is dispatched. Carries task framing into the agent's orchestrator without flow amendment."
	Website      = "https://www.flomation.co"
	Icon         = "list-check+bolt"
	Date         = "22/06/2026"
	Type         = core.ActionTypeTrigger
)

// Outputs match the field names user-message triggers (Telegram /
// Slack / channel webhooks) populate. The downstream AI Prompt
// action's `${flow.X}` substitutions resolve the same way whether the
// active trigger is a channel webhook or a plan tick.
//
// Channel-shaped fields: prompt, channel_type, channel_id,
// conversation_history, agent_id, agent_user_id.
//
// Plan-specific fields: plan_id, plan_task_id, plan_task_name,
// plan_task_description, plan_task_inputs, upstream_outputs. These
// are the references the agent's tools (set_output, plan/block) read.
var Outputs = [...]core.Connection{
	// Channel-shaped fields — wire identically to existing channel
	// triggers so the AI Prompt action reads ${flow.prompt} and
	// ${flow.conversation_history} regardless of source.
	{Name: "prompt", Type: core.ConnectionTypeText, Label: "Pre-formatted task framing — wire into AI Prompt's prompt input"},
	{Name: "channel_type", Type: core.ConnectionTypeString, Label: "Always 'plan_task' — distinguishes from user-message channels"},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Empty (plan tasks have no channel)"},
	{Name: "conversation_history", Type: core.ConnectionTypeObject, Label: "Empty array — plan tasks don't see prior conversation"},
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID"},
	{Name: "agent_user_id", Type: core.ConnectionTypeString, Label: "Agent User ID"},

	// Plan-specific fields — the agent's tools reference these to
	// know which task they're progressing.
	{Name: "plan_id", Type: core.ConnectionTypeString, Label: "Plan ID"},
	{Name: "plan_task_id", Type: core.ConnectionTypeString, Label: "Plan Task ID — pass to set_output or plan/block"},
	{Name: "plan_task_name", Type: core.ConnectionTypeString, Label: "Task name"},
	{Name: "plan_task_description", Type: core.ConnectionTypeText, Label: "Task description"},
	{Name: "plan_task_inputs", Type: core.ConnectionTypeObject, Label: "Task inputs (post-substitution from upstream outputs)"},
	{Name: "upstream_outputs", Type: core.ConnectionTypeObject, Label: "Outputs from completed dependency tasks, keyed by task name"},
}

// Execute is invoked when the flow engine reaches this trigger node.
// Same shape as form / channel triggers: route inputs through to
// outputs. The tick endpoint populates execution.data with the field
// values; the flow engine projects those into the inputs slice; this
// Execute reflects them back as outputs so downstream ${flow.X}
// substitution resolves.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, input := range inputs {
		if input == nil {
			continue
		}
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}
	return result, nil
}
