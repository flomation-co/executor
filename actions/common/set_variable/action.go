package set_variable

import (
	"fmt"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Set Variable"
	Description  = "Set a flow-wide variable accessible via ${var.name}"
	Website      = "https://www.flomation.co"
	Icon         = "code"
	Date         = "25/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "name",
		Type:        core.ConnectionTypeString,
		Label:       "Variable Name",
		Placeholder: "my_variable",
		Required:    true,
	},
	{
		Name:        "value",
		Type:        core.ConnectionTypeString,
		Label:       "Value",
		Placeholder: "",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "name",
		Type:  core.ConnectionTypeString,
		Label: "Variable Name",
	},
	{
		Name:  "value",
		Type:  core.ConnectionTypeString,
		Label: "Value",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	nameConn := core.FindConnection("name", inputs)
	valueConn := core.FindConnection("value", inputs)

	n := nameConn.String()
	if n == nil || *n == "" {
		return nil, fmt.Errorf("variable name is required")
	}

	var value interface{} = ""
	if valueConn != nil {
		value = valueConn.Value
	}

	flow.SetVariable(*n, value)

	return map[string]interface{}{
		"name":  *n,
		"value": value,
	}, nil
}
