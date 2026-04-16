package arithmetic_addition

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Addition"
	Description  = "Add two numbers together and return the sum"
	Website      = "https://www.flomation.co"
	Icon         = "plus"
	Date         = "05/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	core.Connection{
		Name:        "a",
		Type:        core.ConnectionTypeInteger,
		Label:       "A",
		Placeholder: "",
	},
	core.Connection{
		Name:        "b",
		Type:        core.ConnectionTypeInteger,
		Label:       "B",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	core.Connection{
		Name:        "answer",
		Type:        core.ConnectionTypeInteger,
		Label:       "X",
		Placeholder: "",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	a := core.FindConnection("a", inputs)
	b := core.FindConnection("b", inputs)

	return map[string]interface{}{
		"answer": *a.Number() + *b.Number(),
	}, nil
}
