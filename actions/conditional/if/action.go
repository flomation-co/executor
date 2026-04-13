package conditional_if

import (
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "If"
	Description  = "Evaluate a condition and branch execution accordingly"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "20/03/2026"
	Type         = core.ActionTypeConditional
)

var Inputs = [...]core.Connection{
	{
		Name:        "value_a",
		Type:        core.ConnectionTypeString,
		Label:       "Value A",
		Placeholder: "Left operand",
		Required:    true,
	},
	{
		Name:     "operator",
		Type:     core.ConnectionTypeString,
		Label:    "Operator",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "equals"},
			{Name: "Not Equals", Value: "not_equals"},
			{Name: "Contains", Value: "contains"},
			{Name: "Greater Than", Value: "greater_than"},
			{Name: "Less Than", Value: "less_than"},
			{Name: "Is Empty", Value: "is_empty"},
			{Name: "Is Not Empty", Value: "is_not_empty"},
		},
	},
	{
		Name:        "value_b",
		Type:        core.ConnectionTypeString,
		Label:       "Value B",
		Placeholder: "Right operand",
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "result",
		Type:  core.ConnectionTypeBoolean,
		Label: "Result",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	operatorConn := core.FindConnection("operator", inputs)
	if operatorConn == nil || operatorConn.String() == nil {
		return nil, fmt.Errorf("operator is required")
	}
	operator := strings.TrimSpace(*operatorConn.String())

	valueAConn := core.FindConnection("value_a", inputs)
	valueBConn := core.FindConnection("value_b", inputs)

	var valueA, valueB string
	if valueAConn != nil && valueAConn.String() != nil {
		valueA = *valueAConn.String()
	}
	if valueBConn != nil && valueBConn.String() != nil {
		valueB = *valueBConn.String()
	}

	var result bool

	switch operator {
	case "equals":
		result = valueA == valueB
	case "not_equals":
		result = valueA != valueB
	case "contains":
		result = strings.Contains(valueA, valueB)
	case "greater_than":
		a, errA := strconv.ParseFloat(valueA, 64)
		b, errB := strconv.ParseFloat(valueB, 64)
		if errA != nil || errB != nil {
			result = false
		} else {
			result = a > b
		}
	case "less_than":
		a, errA := strconv.ParseFloat(valueA, 64)
		b, errB := strconv.ParseFloat(valueB, 64)
		if errA != nil || errB != nil {
			result = false
		} else {
			result = a < b
		}
	case "is_empty":
		result = strings.TrimSpace(valueA) == ""
	case "is_not_empty":
		result = strings.TrimSpace(valueA) != ""
	default:
		return nil, fmt.Errorf("unknown operator: %s", operator)
	}

	return map[string]interface{}{
		"result": result,
	}, nil
}
