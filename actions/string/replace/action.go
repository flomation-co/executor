package string_replace

import (
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Replace"
	Description  = "Replace all occurrences of a substring with another string"
	Website      = "https://www.flomation.co"
	Icon         = "font+pencil"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "value", Type: core.ConnectionTypeString, Label: "Input String", Required: true},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search For", Required: true},
	{Name: "replace", Type: core.ConnectionTypeString, Label: "Replace With"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "result", Type: core.ConnectionTypeString, Label: "Replaced String"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Replacements Made"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	v := str("value", inputs)
	search := str("search", inputs)
	replace := str("replace", inputs)

	count := strings.Count(v, search)
	result := strings.ReplaceAll(v, search, replace)

	return map[string]interface{}{
		"tool_result": result, "result": result, "count": count, "success": true, "error": "",
	}, nil
}

func str(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
