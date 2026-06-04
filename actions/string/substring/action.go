package string_substring

import (
	"fmt"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Substring"
	Description  = "Extract a portion of a string by start and end positions"
	Website      = "https://www.flomation.co"
	Icon         = "font+filter"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "value", Type: core.ConnectionTypeString, Label: "Input String", Required: true},
	{Name: "start", Type: core.ConnectionTypeInteger, Label: "Start Index (0-based)", Placeholder: "0", Required: true},
	{Name: "end", Type: core.ConnectionTypeInteger, Label: "End Index (exclusive, empty = end of string)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "result", Type: core.ConnectionTypeString, Label: "Substring"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	v := str("value", inputs)
	startConn := core.FindConnection("start", inputs)
	endConn := core.FindConnection("end", inputs)

	runes := []rune(v)
	start := 0
	if startConn != nil && startConn.Number() != nil {
		start = int(*startConn.Number())
	}
	end := len(runes)
	if endConn != nil && endConn.Number() != nil {
		end = int(*endConn.Number())
	}

	if start < 0 { start = 0 }
	if end > len(runes) { end = len(runes) }
	if start > end { start = end }

	result := string(runes[start:end])
	return map[string]interface{}{
		"tool_result": result, "result": result, "success": true, "error": "",
	}, nil
}

func str(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil { return "" }
	return *c.String()
}

func init() { _ = fmt.Sprintf }
