// Package subflow_invoke calls a named sub-flow subroutine. The engine
// intercepts this node's execution and dispatches to the matching
// subflow/begin node, executing its chain and returning the results.
package subflow_invoke

import (
	"fmt"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Invoke Sub-Flow"
	Description  = "Call a named sub-flow subroutine and return its results"
	Website      = "https://www.flomation.co"
	Icon         = "share-from-square"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

// SubFlowNameKey is the output key the engine checks to trigger sub-flow dispatch.
const SubFlowNameKey = "__subflow_name"

var Inputs = [...]core.Connection{
	{
		Name:        "sub_flow_name",
		Type:        core.ConnectionTypeString,
		Label:       "Sub-Flow Name",
		Placeholder: "my_subroutine",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	nameConn := core.FindConnection("sub_flow_name", inputs)
	if nameConn == nil || nameConn.String() == nil || *nameConn.String() == "" {
		return map[string]interface{}{
			"tool_result": "sub_flow_name is required",
			"success":     false,
			"error":       "sub_flow_name is required",
		}, nil
	}

	name := *nameConn.String()

	// Collect all parameter inputs (everything except sub_flow_name)
	// to pass to the Begin node.
	outputs := map[string]interface{}{
		SubFlowNameKey: name,
		"tool_result":  fmt.Sprintf("Invoking sub-flow '%s'", name),
		"success":      true,
		"error":        "",
	}
	for _, inp := range inputs {
		if inp.Name == "sub_flow_name" {
			continue
		}
		if inp.Value != nil {
			outputs[inp.Name] = inp.Value
		}
	}

	// The engine intercepts SubFlowNameKey in executeNodeChildren
	// and dispatches to the matching Begin Sub-Flow node.
	return outputs, nil
}
