// Package pause suspends the flow execution until manually resumed.
// The executor serialises a checkpoint and exits; the runner reports
// the suspended state to the API. A user can then resume the execution
// from the UI or via the API.
package pause

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Pause"
	Description  = "Suspend execution until manually resumed"
	Website      = "https://www.flomation.co"
	Icon         = "pause"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "reason",
		Type:        core.ConnectionTypeString,
		Label:       "Reason",
		Placeholder: "Awaiting manual approval",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "suspended", Type: core.ConnectionTypeBoolean, Label: "Was Suspended"},
	{Name: "reason", Type: core.ConnectionTypeString, Label: "Reason"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	reason := "Manual pause"
	if r := core.FindConnection("reason", inputs); r != nil && r.String() != nil && *r.String() != "" {
		reason = *r.String()
	}

	// On resume, this node re-executes but should NOT suspend again —
	// just pass through so children can execute.
	if flow.IsResumedNode(node.ID) {
		return map[string]interface{}{
			"tool_result": "Resumed: " + reason,
			"suspended":   false,
			"reason":      reason,
		}, nil
	}

	flow.Suspend(&core.SuspendInfo{
		NodeID: node.ID,
		Reason: "manual",
	})

	return map[string]interface{}{
		"tool_result": reason,
		"suspended":   true,
		"reason":      reason,
	}, core.ErrSuspended
}
