// Package array_length returns the number of items in an array input.
package array_length

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Array Length"
	Description  = "Returns the number of items in an array. Optionally walk a dotted path into a JSON object first (e.g. path=\"data.items\")."
	Website      = "https://www.flomation.co"
	Icon         = "list+hashtag"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:     "array",
		Type:     core.ConnectionTypeObject,
		Label:    "Array (or JSON object containing an array — see Path)",
		Required: true,
	},
	{
		Name: "path",
		Type: core.ConnectionTypeString,
		// Optional. When the input is an object that wraps an
		// array (typical of REST APIs like `{"items":[...],
		// "total":N}` or `{"data":{"results":[...]}}`), set this
		// to the dotted route into the array — e.g. "items" or
		// "data.results". Empty path = treat the input itself
		// as the array (existing behaviour).
		Label:       "Path into the array (optional, dotted form like \"data.items\")",
		Placeholder: "items",
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
		return errorResult("array is required", "No array provided"), nil
	}

	// Read the optional path (dotted form). Empty means "the
	// input IS the array" — preserves original behaviour.
	var path string
	if c := core.FindConnection("path", inputs); c != nil && c.String() != nil {
		path = strings.TrimSpace(*c.String())
	}

	// Normalise the raw input to a generic value we can navigate.
	// Three sources:
	//   1. Already-Go arrays (upstream action's typed output)
	//   2. JSON strings (Web/HTTP response bodies, hand-typed JSON)
	//   3. Already-Go objects (when path is set against a typed map)
	var value interface{}
	switch v := arrayConn.Value.(type) {
	case []interface{}, []map[string]interface{}, map[string]interface{}:
		value = v
	case string:
		if err := json.Unmarshal([]byte(v), &value); err != nil {
			return errorResult(
				"input is not valid JSON",
				fmt.Sprintf("Value is not parseable as JSON: %s", truncate(v, 100)),
			), nil
		}
	default:
		return errorResult(
			fmt.Sprintf("expected array or object, got %T", arrayConn.Value),
			fmt.Sprintf("Unsupported type: %T", arrayConn.Value),
		), nil
	}

	// Walk the path if one was supplied. Each segment descends
	// into a map field; integer-shaped segments descend into an
	// array index, supporting paths like "data.results.0.items".
	if path != "" {
		walked, err := walkPath(value, path)
		if err != nil {
			return errorResult(err.Error(), "Path resolution failed: "+err.Error()), nil
		}
		value = walked
	}

	// Final value must be an array — count it.
	length, ok := arrayLengthOf(value)
	if !ok {
		return errorResult(
			fmt.Sprintf("value at path %q is not an array (got %T)", path, value),
			fmt.Sprintf("Not an array (got %T)", value),
		), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%d", length),
		"length":      length,
		"success":     true,
		"error":       "",
	}, nil
}

// walkPath descends into a generic value via a dotted path. Map
// segments are matched as keys; numeric segments descend into
// array indices. Returns an error with the failing segment when
// the route doesn't exist, so the caller can surface "couldn't
// find data.items in the response" rather than a vague nil.
func walkPath(value interface{}, path string) (interface{}, error) {
	segments := strings.Split(path, ".")
	current := value
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		switch v := current.(type) {
		case map[string]interface{}:
			next, ok := v[seg]
			if !ok {
				return nil, fmt.Errorf("path segment %q not found in object at %q", seg, strings.Join(segments[:i+1], "."))
			}
			current = next
		case []interface{}:
			idx, err := strconv.Atoi(seg)
			if err != nil {
				return nil, fmt.Errorf("path segment %q is not a numeric index — got an array at %q", seg, strings.Join(segments[:i+1], "."))
			}
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index %d out of bounds (length %d) at %q", idx, len(v), strings.Join(segments[:i+1], "."))
			}
			current = v[idx]
		default:
			return nil, fmt.Errorf("can't descend into %T at segment %q (path %q)", current, seg, strings.Join(segments[:i+1], "."))
		}
	}
	return current, nil
}

// arrayLengthOf returns the length of an array-shaped value. Returns
// ok=false for non-array values so the caller can surface a clear
// error rather than silently counting object keys or string runes.
func arrayLengthOf(value interface{}) (int, bool) {
	switch v := value.(type) {
	case []interface{}:
		return len(v), true
	case []map[string]interface{}:
		return len(v), true
	}
	return 0, false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func errorResult(errMsg, toolResult string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": toolResult,
		"length":      0,
		"success":     false,
		"error":       errMsg,
	}
}
