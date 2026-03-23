package webhook

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Webhook Trigger"
	Description  = "Triggers a flow when an HTTP webhook is received"
	Website      = "https://www.flomation.co"
	Icon         = "globe"
	Date         = "23/03/2026"
	Type         = core.ActionTypeTrigger
)

var Outputs = [...]core.Connection{
	{Name: "body", Type: core.ConnectionTypeString, Label: "Request Body"},
	{Name: "method", Type: core.ConnectionTypeString, Label: "HTTP Method"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}
