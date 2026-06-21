// Package array_length returns the number of items in an array input.
package array_length

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Array Length"
	Description  = "Returns the number of items in an array"
	Website      = "https://www.flomation.co"
	Icon         = "list+hashtag"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:     "array",
		Type:     core.ConnectionTypeObject,
		Label:    "Array",
		Required: true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "length", Type: core.ConnectionTypeInteger, Label: "Length"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	arrayConn := core.FindConnection("array", inputs)
	if arrayConn == nil || arrayConn.Value == nil {
		return map[string]interface{}{
			"tool_result": "No array provided",
			"length":      0,
			"success":     false,
			"error":       "array is required",
		}, nil
	}

	length := 0
	switch v := arrayConn.Value.(type) {
	case []interface{}:
		length = len(v)
	case []map[string]interface{}:
		length = len(v)
	case string:
		// Try parsing JSON string as array
		var arr []interface{}
		if err := json.Unmarshal([]byte(v), &arr); err == nil {
			length = len(arr)
		} else {
			return map[string]interface{}{
				"tool_result": fmt.Sprintf("Value is not an array: %s", v[:min(len(v), 100)]),
				"length":      0,
				"success":     false,
				"error":       "input is not an array",
			}, nil
		}
	default:
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Unsupported type: %T", arrayConn.Value),
			"length":      0,
			"success":     false,
			"error":       fmt.Sprintf("expected array, got %T", arrayConn.Value),
		}, nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%d", length),
		"length":      length,
		"success":     true,
		"error":       "",
	}, nil
}
