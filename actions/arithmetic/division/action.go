package arithmetic_division

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Maths | Division"
	Description  = "Arithmetic Functions"
	Website      = "https://www.flomation.co"
	Icon         = "divide"
	Date         = "05/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	core.Connection{
		Name:        "numerator",
		Type:        core.ConnectionTypeInteger,
		Label:       "Numerator",
		Placeholder: "",
	},
	core.Connection{
		Name:        "denominator",
		Type:        core.ConnectionTypeInteger,
		Label:       "Denominator",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	core.Connection{
		Name:        "answer",
		Type:        core.ConnectionTypeInteger,
		Label:       "X",
		Placeholder: "",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	a := core.FindConnection("numerator", inputs)
	b := core.FindConnection("denominator", inputs)

	return map[string]interface{}{
		"answer": *a.Number() / *b.Number(),
	}, nil
}
