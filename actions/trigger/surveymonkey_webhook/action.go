package surveymonkey_webhook

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "SurveyMonkey Webhook Trigger"
	Description  = "Triggers a flow when a SurveyMonkey webhook event is received."
	Website      = "https://www.flomation.co"
	Icon         = "clipboard-list"
	Date         = "11/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "secret", Type: core.ConnectionTypeSecret, Label: "Webhook Signing Secret", Placeholder: "${secrets.surveymonkey_webhook_secret}"},
	{Name: "survey_id", Type: core.ConnectionTypeString, Label: "Survey ID Filter", Placeholder: "123456789 (blank = any survey)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "object_type", Type: core.ConnectionTypeString, Label: "Object Type"},
	{Name: "object_id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "survey_id", Type: core.ConnectionTypeString, Label: "Survey ID"},
	{Name: "response_id", Type: core.ConnectionTypeString, Label: "Response ID"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they hold the signing secret and the survey filter.
var configInputs = map[string]bool{
	"secret":    true,
	"__node_id": true,
}

// Execute echoes the event data Launch resolved from the verified SurveyMonkey
// webhook into the flow's outputs. Launch performs the signature verification
// and payload parsing; this node just surfaces the fields. The trigger-type
// string registered with Launch is "surveymonkey-webhook".
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing SurveyMonkey webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	if _, ok := result["tool_result"]; !ok {
		if et, ok := result["event_type"].(string); ok && et != "" {
			result["tool_result"] = "SurveyMonkey event: " + et
		} else {
			result["tool_result"] = "SurveyMonkey webhook received"
		}
	}
	return result, nil
}
