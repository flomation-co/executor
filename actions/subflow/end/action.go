// Package subflow_end is the optional return point of a sub-flow subroutine.
// It collects its inputs and returns them to the Invoke node. The engine
// stops traversal at this node — no children are executed.
package subflow_end

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "End Sub-Flow"
	Description  = "Return point of a sub-flow. Outputs are returned to the Invoke node."
	Website      = "https://www.flomation.co"
	Icon         = "stop"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:  "return_value",
		Type:  core.ConnectionTypeString,
		Label: "Return Value (optional explicit return; otherwise all parent outputs are returned)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	outputs := map[string]interface{}{}

	// Collect all inputs as return values
	for _, inp := range inputs {
		if inp.Value != nil {
			outputs[inp.Name] = inp.Value
		}
	}

	// Build a summary for tool_result
	if b, err := json.Marshal(outputs); err == nil {
		outputs["tool_result"] = fmt.Sprintf("Sub-flow returned: %s", string(b))
	} else {
		outputs["tool_result"] = "Sub-flow returned"
	}

	return outputs, nil
}
