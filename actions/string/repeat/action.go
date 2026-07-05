package string_repeat

import (
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Repeat"
	Description  = "Repeat a string a given number of times"
	Website      = "https://www.flomation.co"
	Icon         = "i-cursor+repeat"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "value", Type: core.ConnectionTypeString, Label: "Input String", Required: true},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Repeat Count", Placeholder: "2", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "result", Type: core.ConnectionTypeString, Label: "Repeated String"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	v := str("value", inputs)
	count := 1
	if c := core.FindConnection("count", inputs); c != nil && c.Number() != nil {
		count = int(*c.Number())
	}
	if count < 0 {
		count = 0
	}
	if count > 10000 {
		count = 10000
	}

	result := strings.Repeat(v, count)
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
