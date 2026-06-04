package string_join

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Join"
	Description  = "Join an array of strings with a separator"
	Website      = "https://www.flomation.co"
	Icon         = "font+link"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "array", Type: core.ConnectionTypeObject, Label: "Array of Strings", Required: true},
	{Name: "separator", Type: core.ConnectionTypeString, Label: "Separator", Placeholder: ", "},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "result", Type: core.ConnectionTypeString, Label: "Joined String"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	sep := str("separator", inputs)
	if sep == "" { sep = ", " }

	arrayConn := core.FindConnection("array", inputs)
	if arrayConn == nil || arrayConn.Value == nil {
		return map[string]interface{}{
			"tool_result": "", "result": "", "success": false, "error": "array is required",
		}, nil
	}

	var parts []string
	switch v := arrayConn.Value.(type) {
	case []interface{}:
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
	case []string:
		parts = v
	case string:
		var arr []interface{}
		if err := json.Unmarshal([]byte(v), &arr); err == nil {
			for _, item := range arr {
				parts = append(parts, fmt.Sprintf("%v", item))
			}
		} else {
			parts = []string{v}
		}
	}

	result := strings.Join(parts, sep)
	return map[string]interface{}{
		"tool_result": result, "result": result, "success": true, "error": "",
	}, nil
}

func str(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil { return "" }
	return *c.String()
}
