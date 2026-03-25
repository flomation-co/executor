package conditional_while

import (
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "While"
	Description  = "Loop while a condition is true"
	Website      = "https://www.flomation.co"
	Icon         = "repeat"
	Date         = "25/03/2026"
	Type         = core.ActionTypeLoop
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
	{
		Name:        "max_iterations",
		Type:        core.ConnectionTypeInteger,
		Label:       "Max Iterations",
		Placeholder: "1000",
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "result",
		Type:  core.ConnectionTypeBoolean,
		Label: "Condition Result",
	},
	{
		Name:  "iterations",
		Type:  core.ConnectionTypeInteger,
		Label: "Iteration Count",
	},
	{
		Name:  "max_iterations",
		Type:  core.ConnectionTypeInteger,
		Label: "Max Iterations",
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

	maxIter := int64(1000)
	maxIterConn := core.FindConnection("max_iterations", inputs)
	if maxIterConn != nil && maxIterConn.String() != nil {
		if v, err := strconv.ParseInt(*maxIterConn.String(), 10, 64); err == nil && v > 0 {
			maxIter = v
		}
	}

	result := evaluateCondition(operator, valueA, valueB)

	return map[string]interface{}{
		"result":         result,
		"iterations":     int64(0),
		"max_iterations": maxIter,
	}, nil
}

func evaluateCondition(operator, valueA, valueB string) bool {
	switch operator {
	case "equals":
		return valueA == valueB
	case "not_equals":
		return valueA != valueB
	case "contains":
		return strings.Contains(valueA, valueB)
	case "greater_than":
		a, errA := strconv.ParseFloat(valueA, 64)
		b, errB := strconv.ParseFloat(valueB, 64)
		if errA != nil || errB != nil {
			return false
		}
		return a > b
	case "less_than":
		a, errA := strconv.ParseFloat(valueA, 64)
		b, errB := strconv.ParseFloat(valueB, 64)
		if errA != nil || errB != nil {
			return false
		}
		return a < b
	case "is_empty":
		return strings.TrimSpace(valueA) == ""
	case "is_not_empty":
		return strings.TrimSpace(valueA) != ""
	default:
		return false
	}
}
