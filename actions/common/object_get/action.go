// Package object_get extracts one or more fields from an object or JSON string.
// Supports dot-notation for nested access (e.g. "address.city").
package object_get

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Object Get Field"
	Description  = "Extract a field from an object by name, with dot-notation for nested access"
	Website      = "https://www.flomation.co"
	Icon         = "magnifying-glass"
	Date         = "22/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:     "object",
		Type:     core.ConnectionTypeObject,
		Label:    "Object",
		Required: true,
	},
	{
		Name:        "field",
		Type:        core.ConnectionTypeString,
		Label:       "Field Name",
		Placeholder: "name or address.city",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "value", Type: core.ConnectionTypeObject, Label: "Field Value"},
	{Name: "value_string", Type: core.ConnectionTypeString, Label: "Field Value (string)"},
	{Name: "found", Type: core.ConnectionTypeBoolean, Label: "Found"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	objConn := core.FindConnection("object", inputs)
	if objConn == nil || objConn.Value == nil {
		return errResult("object is required")
	}

	fieldConn := core.FindConnection("field", inputs)
	if fieldConn == nil || fieldConn.String() == nil || *fieldConn.String() == "" {
		return errResult("field is required")
	}
	field := *fieldConn.String()

	obj, err := toMap(objConn.Value)
	if err != nil {
		return errResult(err.Error())
	}

	// Walk dot-notation path
	val, found := resolve(obj, field)
	if !found {
		return map[string]interface{}{
			"tool_result":  fmt.Sprintf("field '%s' not found", field),
			"value":        nil,
			"value_string": "",
			"found":        false,
			"success":      true,
			"error":        "",
		}, nil
	}

	var valStr string
	if s, ok := val.(string); ok {
		valStr = s
	} else if b, err := json.Marshal(val); err == nil {
		valStr = string(b)
	} else {
		valStr = fmt.Sprintf("%v", val)
	}

	return map[string]interface{}{
		"tool_result":  valStr,
		"value":        val,
		"value_string": valStr,
		"found":        true,
		"success":      true,
		"error":        "",
	}, nil
}

func resolve(obj map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var current interface{} = obj

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}

func toMap(v interface{}) (map[string]interface{}, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		return val, nil
	case string:
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(val), &m); err != nil {
			return nil, fmt.Errorf("value is not an object: %s", val[:min(len(val), 100)])
		}
		return m, nil
	default:
		return nil, fmt.Errorf("expected object, got %T", v)
	}
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":  msg,
		"value":        nil,
		"value_string": "",
		"found":        false,
		"success":      false,
		"error":        msg,
	}, nil
}
