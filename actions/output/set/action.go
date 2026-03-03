package output

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Set Output"
	Description  = "Pass data out from a Flow"
	Website      = "https://www.flomation.co"
	Icon         = "dollar-sign"
	Date         = "27/11/2025"
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	name := core.FindConnection("name", inputs)
	value := core.FindConnection("value", inputs)

	result := false
	n := name.String()
	if n != nil && value != nil {
		flow.SetOutput(*n, value.Value)
		result = true
	}

	return map[string]interface{}{
		"set": result,
	}, nil
}
