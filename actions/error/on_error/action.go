package error_on_error

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "On Error"
	Description  = "Executes when a flow error occurs"
	Website      = "https://www.flomation.co"
	Icon         = "triangle-exclamation"
	Date         = "25/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{}

var Outputs = [...]core.Connection{
	{
		Name:  "error_message",
		Type:  core.ConnectionTypeString,
		Label: "Error Message",
	},
	{
		Name:  "error_node_id",
		Type:  core.ConnectionTypeString,
		Label: "Failed Node ID",
	},
	{
		Name:  "error_node_label",
		Type:  core.ConnectionTypeString,
		Label: "Failed Node Label",
	},
	{
		Name:  "error_node_type",
		Type:  core.ConnectionTypeString,
		Label: "Failed Node Type",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// On Error is a passthrough — error context is injected by the executor
	// via flow.SetNodeResultForTest or direct result injection before execution
	return map[string]interface{}{
		"error_message":    "",
		"error_node_id":    "",
		"error_node_label": "",
		"error_node_type":  "",
	}, nil
}
