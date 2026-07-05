package string_lower_case

import (
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Lower Case"
	Description  = "Convert a string to lower case"
	Website      = "https://www.flomation.co"
	Icon         = "i-cursor+arrow-down"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "value", Type: core.ConnectionTypeString, Label: "Input String", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "result", Type: core.ConnectionTypeString, Label: "Lower Case String"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	v := str("value", inputs)
	result := strings.ToLower(v)
	return map[string]interface{}{
		"tool_result": result, "result": result, "success": true, "error": "",
	}, nil
}

func str(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
