package mailchimp_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Mailchimp Trigger"
	Description  = "Triggers a flow on Mailchimp audience events (subscribe, unsubscribe, profile update, cleaned, email changed, campaign sent) via a webhook."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp"
	Date         = "01/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "Audience (List) ID", Required: true},
	{Name: "events", Type: core.ConnectionTypeString, Label: "Events", Placeholder: "Comma-separated: subscribe,unsubscribe,profile,cleaned,upemail,campaign"},
	{Name: "sources", Type: core.ConnectionTypeString, Label: "Sources", Placeholder: "Comma-separated: user,admin,api"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "Audience (List) ID"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email"},
	{Name: "fired_at", Type: core.ConnectionTypeString, Label: "Fired At"},
	{Name: "body", Type: core.ConnectionTypeObject, Label: "Raw Event Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they hold the secret or internal filter settings. list_id is
// deliberately NOT here: at fire time the launch service injects the event's
// list_id as an input, and it must flow through to the list_id output.
var configInputs = map[string]bool{
	"api_key":   true,
	"events":    true,
	"sources":   true,
	"__node_id": true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Mailchimp webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	result["content"] = buildContentSummary(result)

	return result, nil
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func buildContentSummary(data map[string]interface{}) string {
	eventType := str(data["event_type"])
	email := str(data["email"])
	listID := str(data["list_id"])

	if eventType == "" {
		return "[Mailchimp Event] received"
	}
	if email != "" {
		return fmt.Sprintf("[Mailchimp] %s event for %s on audience %s", eventType, email, listID)
	}
	return fmt.Sprintf("[Mailchimp] %s event on audience %s", eventType, listID)
}
