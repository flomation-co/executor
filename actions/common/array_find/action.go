// Package array_find extracts an item from an array by index or by matching a field value.
package array_find

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Array Find"
	Description  = "Find an item in an array by index or by matching a field value"
	Website      = "https://www.flomation.co"
	Icon         = "magnifying-glass"
	Date         = "22/05/2026"
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
		Name:  "mode",
		Type:  core.ConnectionTypeString,
		Label: "Find Mode",
		Options: []core.ConnectionOption{
			{Name: "By Index", Value: "index"},
			{Name: "First Match", Value: "first"},
			{Name: "All Matches", Value: "all"},
		},
		Required: true,
	},
	{
		Name:        "index",
		Type:        core.ConnectionTypeInteger,
		Label:       "Index (0-based)",
		Placeholder: "0",
		Visible:     &core.VisibleWhen{Field: "mode", Values: []string{"index"}},
	},
	{
		Name:        "field",
		Type:        core.ConnectionTypeString,
		Label:       "Field Name",
		Placeholder: "id",
		Visible:     &core.VisibleWhen{Field: "mode", Values: []string{"first", "all"}},
	},
	{
		Name:  "operator",
		Type:  core.ConnectionTypeString,
		Label: "Operator",
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "equals"},
			{Name: "Contains", Value: "contains"},
			{Name: "Starts With", Value: "starts_with"},
			{Name: "Ends With", Value: "ends_with"},
			{Name: "Not Equals", Value: "not_equals"},
		},
		Visible: &core.VisibleWhen{Field: "mode", Values: []string{"first", "all"}},
	},
	{
		Name:        "value",
		Type:        core.ConnectionTypeString,
		Label:       "Match Value",
		Placeholder: "Value to match against",
		Visible:     &core.VisibleWhen{Field: "mode", Values: []string{"first", "all"}},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "item", Type: core.ConnectionTypeObject, Label: "Matched Item (first or index)"},
	{Name: "items", Type: core.ConnectionTypeObject, Label: "All Matched Items (array)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Match Count"},
	{Name: "found", Type: core.ConnectionTypeBoolean, Label: "Found"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	arrayConn := core.FindConnection("array", inputs)
	if arrayConn == nil || arrayConn.Value == nil {
		return errResult("array is required")
	}

	arr, err := toSlice(arrayConn.Value)
	if err != nil {
		return errResult(err.Error())
	}

	modeConn := core.FindConnection("mode", inputs)
	mode := "index"
	if modeConn != nil && modeConn.String() != nil {
		mode = *modeConn.String()
	}

	switch mode {
	case "index":
		return findByIndex(arr, inputs)
	case "first":
		return findByField(arr, inputs, false)
	case "all":
		return findByField(arr, inputs, true)
	default:
		return errResult(fmt.Sprintf("unknown mode: %s", mode))
	}
}

func findByIndex(arr []interface{}, inputs []*core.Connection) (map[string]interface{}, error) {
	indexConn := core.FindConnection("index", inputs)
	if indexConn == nil || indexConn.Number() == nil {
		return errResult("index is required in index mode")
	}
	idx := int(*indexConn.Number())

	if idx < 0 {
		idx = len(arr) + idx
	}
	if idx < 0 || idx >= len(arr) {
		return errResult(fmt.Sprintf("index %d out of range (array has %d items)", idx, len(arr)))
	}

	item := arr[idx]
	resultStr := marshal(item)

	return map[string]interface{}{
		"tool_result": resultStr,
		"item":        item,
		"items":       []interface{}{item},
		"count":       int64(1),
		"found":       true,
		"success":     true,
		"error":       "",
	}, nil
}

func findByField(arr []interface{}, inputs []*core.Connection, all bool) (map[string]interface{}, error) {
	fieldConn := core.FindConnection("field", inputs)
	if fieldConn == nil || fieldConn.String() == nil || *fieldConn.String() == "" {
		return errResult("field is required for match mode")
	}
	field := *fieldConn.String()

	operatorConn := core.FindConnection("operator", inputs)
	operator := "equals"
	if operatorConn != nil && operatorConn.String() != nil && *operatorConn.String() != "" {
		operator = *operatorConn.String()
	}

	valueConn := core.FindConnection("value", inputs)
	matchValue := ""
	if valueConn != nil && valueConn.String() != nil {
		matchValue = *valueConn.String()
	}

	var matched []interface{}
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		fieldVal := fmt.Sprintf("%v", obj[field])

		if matches(fieldVal, operator, matchValue) {
			matched = append(matched, item)
			if !all {
				break
			}
		}
	}

	if len(matched) == 0 {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("No items matched %s %s '%s'", field, operator, matchValue),
			"item":        nil,
			"items":       []interface{}{},
			"count":       int64(0),
			"found":       false,
			"success":     true,
			"error":       "",
		}, nil
	}

	firstItem := matched[0]
	resultStr := marshal(firstItem)
	if all {
		resultStr = fmt.Sprintf("Found %d matches", len(matched))
	}

	return map[string]interface{}{
		"tool_result": resultStr,
		"item":        firstItem,
		"items":       matched,
		"count":       int64(len(matched)),
		"found":       true,
		"success":     true,
		"error":       "",
	}, nil
}

func matches(fieldVal, operator, matchValue string) bool {
	switch operator {
	case "equals":
		return fieldVal == matchValue
	case "not_equals":
		return fieldVal != matchValue
	case "contains":
		return strings.Contains(strings.ToLower(fieldVal), strings.ToLower(matchValue))
	case "starts_with":
		return strings.HasPrefix(strings.ToLower(fieldVal), strings.ToLower(matchValue))
	case "ends_with":
		return strings.HasSuffix(strings.ToLower(fieldVal), strings.ToLower(matchValue))
	default:
		return fieldVal == matchValue
	}
}

func toSlice(v interface{}) ([]interface{}, error) {
	switch val := v.(type) {
	case []interface{}:
		return val, nil
	case []map[string]interface{}:
		arr := make([]interface{}, len(val))
		for i, item := range val {
			arr[i] = item
		}
		return arr, nil
	case string:
		var arr []interface{}
		if err := json.Unmarshal([]byte(val), &arr); err != nil {
			return nil, fmt.Errorf("value is not an array: %s", val[:min(len(val), 100)])
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("expected array, got %T", v)
	}
}

func marshal(v interface{}) string {
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"item":        nil,
		"items":       []interface{}{},
		"count":       int64(0),
		"found":       false,
		"success":     false,
		"error":       msg,
	}, nil
}
