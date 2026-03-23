package form

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Form Trigger"
	Description  = "Triggers a flow when a form is submitted"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines"
	Date         = "23/03/2026"
	Type         = core.ActionTypeTrigger
)

var Outputs = [...]core.Connection{
	{Name: "form_data", Type: core.ConnectionTypeString, Label: "Form Data"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing form trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}
