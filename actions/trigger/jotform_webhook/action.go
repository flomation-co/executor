package jotform_webhook

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "JotForm Webhook Trigger"
	Description  = "Triggers a flow when a JotForm form submission webhook is received."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list"
	Date         = "11/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "secret", Type: core.ConnectionTypeSecret, Label: "Shared Token", Placeholder: "${secrets.jotform_webhook_secret}"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID Filter", Placeholder: "231234567890 (blank = any form)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID"},
	{Name: "submission_id", Type: core.ConnectionTypeString, Label: "Submission ID"},
	{Name: "answers", Type: core.ConnectionTypeString, Label: "Answers (JSON)"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they hold the shared token and the form filter.
var configInputs = map[string]bool{
	"secret":    true,
	"__node_id": true,
}

// Execute echoes the event data Launch resolved from the verified JotForm
// webhook into the flow's outputs. Launch performs the shared-token check and
// payload parsing; this node just surfaces the fields. The trigger-type string
// registered with Launch is "jotform-webhook".
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing JotForm webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	if _, ok := result["tool_result"]; !ok {
		if et, ok := result["event_type"].(string); ok && et != "" {
			result["tool_result"] = "JotForm event: " + et
		} else {
			result["tool_result"] = "JotForm webhook received"
		}
	}
	return result, nil
}
