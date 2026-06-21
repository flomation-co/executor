package string_contains

import (
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Contains"
	Description  = "Check if a string contains a substring"
	Website      = "https://www.flomation.co"
	Icon         = "font+magnifying-glass"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "value", Type: core.ConnectionTypeString, Label: "Input String", Required: true},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search For", Required: true},
	{Name: "case_sensitive", Type: core.ConnectionTypeBoolean, Label: "Case Sensitive"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "result", Type: core.ConnectionTypeBoolean, Label: "Contains"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	v := str("value", inputs)
	search := str("search", inputs)

	caseSensitive := true
	if cs := core.FindConnection("case_sensitive", inputs); cs != nil && cs.Boolean() != nil {
		caseSensitive = *cs.Boolean()
	}

	var result bool
	if caseSensitive {
		result = strings.Contains(v, search)
	} else {
		result = strings.Contains(strings.ToLower(v), strings.ToLower(search))
	}

	resultStr := "false"
	if result {
		resultStr = "true"
	}

	return map[string]interface{}{
		"tool_result": resultStr, "result": result, "success": true, "error": "",
	}, nil
}

func str(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
