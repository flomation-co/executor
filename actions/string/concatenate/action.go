package string_concatenate

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Concatenate"
	Description  = "Join two or more strings together"
	Website      = "https://www.flomation.co"
	Icon         = "font+link"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "a", Type: core.ConnectionTypeString, Label: "First String", Required: true},
	{Name: "b", Type: core.ConnectionTypeString, Label: "Second String"},
	{Name: "c", Type: core.ConnectionTypeString, Label: "Third String (optional)"},
	{Name: "d", Type: core.ConnectionTypeString, Label: "Fourth String (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "result", Type: core.ConnectionTypeString, Label: "Concatenated String"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	result := str("a", inputs) + str("b", inputs) + str("c", inputs) + str("d", inputs)
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
