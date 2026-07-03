package calendly_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly Webhook Trigger"
	Description  = "Triggers a flow when a Calendly event occurs (invitee created/canceled, no-shows, routing form submissions). The webhook subscription is registered with Calendly automatically."
	Website      = "https://www.flomation.co"
	Icon         = "calendly"
	Date         = "03/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token (used to register the webhook)", Required: true},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Options: []core.ConnectionOption{
		{Name: "User", Value: "user"},
		{Name: "Organization", Value: "organization"},
	}},
	{Name: "events", Type: core.ConnectionTypeMultiSelect, Label: "Events", Required: true, Options: []core.ConnectionOption{
		{Name: "Invitee Created (event booked)", Value: "invitee.created"},
		{Name: "Invitee Canceled (event canceled)", Value: "invitee.canceled"},
		{Name: "Invitee No-Show Marked", Value: "invitee_no_show.created"},
		{Name: "Invitee No-Show Unmarked", Value: "invitee_no_show.deleted"},
		{Name: "Routing Form Submitted", Value: "routing_form_submission.created"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "payload", Type: core.ConnectionTypeObject, Label: "Payload"},
	{Name: "invitee_name", Type: core.ConnectionTypeString, Label: "Invitee Name"},
	{Name: "invitee_email", Type: core.ConnectionTypeString, Label: "Invitee Email"},
	{Name: "event_start_time", Type: core.ConnectionTypeString, Label: "Event Start Time"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Created At"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they contain the access token or internal registration settings.
var configInputs = map[string]bool{
	"access_token": true,
	"scope":        true,
	"events":       true,
	"__node_id":    true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Calendly webhook trigger")

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
	event := str(data["event"])
	name := str(data["invitee_name"])
	email := str(data["invitee_email"])

	if event == "" {
		event = "event"
	}
	who := name
	if who == "" {
		who = email
	}
	if who != "" {
		return fmt.Sprintf("[Calendly] %s — %s", event, who)
	}
	return fmt.Sprintf("[Calendly] %s", event)
}
