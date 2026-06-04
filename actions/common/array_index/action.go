// Package array_index extracts a single item from an array by index.
package array_index

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Array Index"
	Description  = "Extract a single item from an array by its index (0-based)"
	Website      = "https://www.flomation.co"
	Icon         = "list+magnifying-glass"
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
	{
		Name:        "index",
		Type:        core.ConnectionTypeInteger,
		Label:       "Index (0-based)",
		Placeholder: "0",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "item", Type: core.ConnectionTypeObject, Label: "Item"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	arrayConn := core.FindConnection("array", inputs)
	if arrayConn == nil || arrayConn.Value == nil {
		return errResult("array is required")
	}

	indexConn := core.FindConnection("index", inputs)
	if indexConn == nil || indexConn.Number() == nil {
		return errResult("index is required")
	}
	idx := int(*indexConn.Number())

	var arr []interface{}
	switch v := arrayConn.Value.(type) {
	case []interface{}:
		arr = v
	case []map[string]interface{}:
		for _, item := range v {
			arr = append(arr, item)
		}
	case string:
		if err := json.Unmarshal([]byte(v), &arr); err != nil {
			return errResult(fmt.Sprintf("value is not an array: %s", v[:min(len(v), 100)]))
		}
	default:
		return errResult(fmt.Sprintf("expected array, got %T", arrayConn.Value))
	}

	// Support negative indexing (Python-style)
	if idx < 0 {
		idx = len(arr) + idx
	}

	if idx < 0 || idx >= len(arr) {
		return errResult(fmt.Sprintf("index %d out of range (array has %d items)", idx, len(arr)))
	}

	item := arr[idx]
	resultStr := fmt.Sprintf("%v", item)
	if b, err := json.Marshal(item); err == nil {
		resultStr = string(b)
	}

	return map[string]interface{}{
		"tool_result": resultStr,
		"item":        item,
		"success":     true,
		"error":       "",
	}, nil
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"item":        nil,
		"success":     false,
		"error":       msg,
	}, nil
}