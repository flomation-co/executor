package arithmetic_division

import (
	"fmt"

	core "flomation.app/automate/executor"
	arithmetic "flomation.app/automate/executor/actions/arithmetic"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Division"
	Description  = "Divide one number by another and return the quotient. Accepts integers or decimals."
	Website      = "https://www.flomation.co"
	Icon         = "divide"
	Date         = "05/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "numerator",
		Type:        core.ConnectionTypeString,
		Label:       "Numerator",
		Placeholder: "e.g. 100 or 3.14",
	},
	{
		Name:        "denominator",
		Type:        core.ConnectionTypeString,
		Label:       "Denominator",
		Placeholder: "e.g. 4 or 0.40 (must be non-zero)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "answer", Type: core.ConnectionTypeString, Label: "Quotient (decimal)"},
	{Name: "answer_int", Type: core.ConnectionTypeInteger, Label: "Quotient (rounded to integer)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	a, err := arithmetic.ParseNumber(core.FindConnection("numerator", inputs), "numerator")
	if err != nil {
		return nil, err
	}
	b, err := arithmetic.ParseNumber(core.FindConnection("denominator", inputs), "denominator")
	if err != nil {
		return nil, err
	}

	if b == 0 {
		return nil, fmt.Errorf("division: denominator cannot be zero")
	}

	answer := a / b
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%s ÷ %s = %s", arithmetic.FormatNumber(a), arithmetic.FormatNumber(b), arithmetic.FormatNumber(answer)),
		"answer":      arithmetic.FormatNumber(answer),
		"answer_int":  int64(answer),
	}, nil
}
