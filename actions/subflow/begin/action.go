// Package subflow_begin is the entry point of a reusable sub-flow subroutine.
// It has no input handle — it only executes when invoked by a subflow/invoke node.
// The name property identifies which sub-flow this begins.
package subflow_begin

import (
	"fmt"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Begin Sub-Flow"
	Description  = "Entry point of a reusable sub-flow. Give it a name and connect actions below it."
	Website      = "https://www.flomation.co"
	Icon         = "play"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "name",
		Type:        core.ConnectionTypeString,
		Label:       "Sub-Flow Name",
		Placeholder: "my_subroutine",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Sub-Flow Name"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	nameConn := core.FindConnection("name", inputs)
	if nameConn == nil || nameConn.String() == nil || *nameConn.String() == "" {
		return nil, fmt.Errorf("sub-flow name is required")
	}
	name := *nameConn.String()

	// Passthrough: echo all inputs as outputs so downstream nodes
	// can reference parameters passed by the Invoke node.
	outputs := map[string]interface{}{
		"tool_result": fmt.Sprintf("Sub-flow '%s' started", name),
		"name":        name,
	}

	// Forward any additional inputs (dynamic parameters from Invoke)
	for _, inp := range inputs {
		if inp.Name == "name" {
			continue
		}
		if inp.Value != nil {
			outputs[inp.Name] = inp.Value
		}
	}

	return outputs, nil
}
