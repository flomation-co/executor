package arithmetic_subtraction

import (
	"fmt"

	core "flomation.app/automate/executor"
	arithmetic "flomation.app/automate/executor/actions/arithmetic"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Subtraction"
	Description  = "Subtract one number from another and return the difference. Accepts integers or decimals."
	Website      = "https://www.flomation.co"
	Icon         = "minus"
	Date         = "05/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "a",
		Type:        core.ConnectionTypeString,
		Label:       "A",
		Placeholder: "e.g. 10 or 3.14",
	},
	{
		Name:        "b",
		Type:        core.ConnectionTypeString,
		Label:       "B",
		Placeholder: "e.g. 4 or 0.40",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "answer", Type: core.ConnectionTypeString, Label: "Difference (decimal)"},
	{Name: "answer_int", Type: core.ConnectionTypeInteger, Label: "Difference (rounded to integer)"},
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

	answer := a - b
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%s − %s = %s", arithmetic.FormatNumber(a), arithmetic.FormatNumber(b), arithmetic.FormatNumber(answer)),
		"answer":      arithmetic.FormatNumber(answer),
		"answer_int":  int64(answer),
	}, nil
}
