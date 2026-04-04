package switchaction

import (
	"fmt"
	"regexp"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Switch"
	Description  = "Route execution to one of multiple branches based on matching conditions"
	Website      = "https://www.flomation.co"
	Icon         = "route"
	Date         = "04/04/2026"
	Type         = core.ActionTypeSwitch
)

var Inputs = [...]core.Connection{
	{
		Name:        "value",
		Type:        core.ConnectionTypeString,
		Label:       "Value",
		Placeholder: "The value to evaluate",
		Required:    true,
	},
	{
		Name:  "operator",
		Type:  core.ConnectionTypeString,
		Label: "Operator",
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "equals"},
			{Name: "Contains", Value: "contains"},
			{Name: "Starts With", Value: "starts_with"},
			{Name: "Ends With", Value: "ends_with"},
			{Name: "Regex", Value: "regex"},
		},
	},
	{
		Name:        "cases",
		Type:        core.ConnectionTypeKeyValueArray,
		Label:       "Cases",
		Placeholder: "Case label → condition value",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "matched_case", Type: core.ConnectionTypeString, Label: "Matched Case"},
	{Name: "value", Type: core.ConnectionTypeString, Label: "Value"},
	{Name: "matched", Type: core.ConnectionTypeBoolean, Label: "Has Match"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	valueConn := core.FindConnection("value", inputs)
	if valueConn == nil || valueConn.String() == nil {
		return nil, fmt.Errorf("value is required")
	}
	value := *valueConn.String()

	operator := "equals"
	operatorConn := core.FindConnection("operator", inputs)
	if operatorConn != nil && operatorConn.String() != nil && *operatorConn.String() != "" {
		operator = *operatorConn.String()
	}

	casesConn := core.FindConnection("cases", inputs)
	if casesConn == nil {
		return nil, fmt.Errorf("cases are required")
	}

	cases := casesConn.KeyValuePairs()
	if len(cases) == 0 {
		return map[string]interface{}{
			"matched_case": "default",
			"value":        value,
			"matched":      false,
		}, nil
	}

	// Evaluate each case in order — first match wins
	for i, c := range cases {
		if matchCondition(value, c.Value, operator) {
			handle := fmt.Sprintf("case_%d", i)
			return map[string]interface{}{
				"matched_case": handle,
				"value":        value,
				"matched":      true,
			}, nil
		}
	}

	// No match — route to default
	return map[string]interface{}{
		"matched_case": "default",
		"value":        value,
		"matched":      false,
	}, nil
}

func matchCondition(value string, condition string, operator string) bool {
	switch operator {
	case "equals":
		return strings.EqualFold(value, condition)
	case "contains":
		return strings.Contains(strings.ToLower(value), strings.ToLower(condition))
	case "starts_with":
		return strings.HasPrefix(strings.ToLower(value), strings.ToLower(condition))
	case "ends_with":
		return strings.HasSuffix(strings.ToLower(value), strings.ToLower(condition))
	case "regex":
		matched, err := regexp.MatchString(condition, value)
		return err == nil && matched
	default:
		return strings.EqualFold(value, condition)
	}
}
