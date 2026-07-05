package string_trim_start

import (
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Trim Start"
	Description  = "Remove whitespace from the start of a string"
	Website      = "https://www.flomation.co"
	Icon         = "i-cursor+arrow-right"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "value", Type: core.ConnectionTypeString, Label: "Input String", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "result", Type: core.ConnectionTypeString, Label: "Trimmed String"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	v := str("value", inputs)
	result := strings.TrimLeft(v, " \t\n\r")
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
