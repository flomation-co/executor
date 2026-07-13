package typeform_webhook

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Typeform Webhook Trigger"
	Description  = "Triggers a flow when a Typeform form submission webhook is received."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list"
	Date         = "11/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "secret", Type: core.ConnectionTypeSecret, Label: "Webhook Signing Secret", Placeholder: "${secrets.typeform_webhook_secret}", Required: true},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID Filter", Placeholder: "abc123 (blank = any form)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "form_id", Type: core.ConnectionTypeString, Label: "Form ID"},
	{Name: "response_token", Type: core.ConnectionTypeString, Label: "Response Token"},
	{Name: "submitted_at", Type: core.ConnectionTypeString, Label: "Submitted At"},
	{Name: "answers", Type: core.ConnectionTypeString, Label: "Answers (JSON)"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they hold the signing secret and the form filter.
var configInputs = map[string]bool{
	"secret":    true,
	"__node_id": true,
}

// Execute echoes the event data Launch resolved from the verified Typeform
// webhook into the flow's outputs. Launch performs the HMAC signature
// verification and payload parsing; this node just surfaces the fields. The
// trigger-type string registered with Launch is "typeform-webhook".
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Typeform webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	if _, ok := result["tool_result"]; !ok {
		if et, ok := result["event_type"].(string); ok && et != "" {
			result["tool_result"] = "Typeform event: " + et
		} else {
			result["tool_result"] = "Typeform webhook received"
		}
	}
	return result, nil
}
