package arithmetic_multiplication

import (
	"fmt"

	core "flomation.app/automate/executor"
	arithmetic "flomation.app/automate/executor/actions/arithmetic"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Multiplication"
	Description  = "Multiply two numbers together and return the product. Accepts integers or decimals."
	Website      = "https://www.flomation.co"
	Icon         = "xmark"
	Date         = "05/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "a",
		Type:        core.ConnectionTypeString,
		Label:       "A",
		Placeholder: "e.g. 5 or 3.14 or ${parent.distance_miles}",
	},
	{
		Name:        "b",
		Type:        core.ConnectionTypeString,
		Label:       "B",
		Placeholder: "e.g. 0.40",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "answer", Type: core.ConnectionTypeString, Label: "Product (decimal)"},
	{Name: "answer_int", Type: core.ConnectionTypeInteger, Label: "Product (rounded to integer)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	a, err := arithmetic.ParseNumber(core.FindConnection("a", inputs), "a")
	if err != nil {
		return nil, err
	}
	b, err := arithmetic.ParseNumber(core.FindConnection("b", inputs), "b")
	if err != nil {
		return nil, err
	}

	answer := a * b
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%s × %s = %s", arithmetic.FormatNumber(a), arithmetic.FormatNumber(b), arithmetic.FormatNumber(answer)),
		"answer":      arithmetic.FormatNumber(answer),
		"answer_int":  int64(answer),
	}, nil
}
