package sendgrid_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid Webhook Trigger"
	Description  = "Triggers a flow when SendGrid reports an email event — delivered, bounced, opened, clicked, marked as spam, unsubscribed, and more. The event webhook is registered with SendGrid automatically and signed-event verification is enabled."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+bolt"
	Date         = "09/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "SendGrid API key (SendGrid → Settings → API Keys) — used to register the event webhook", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "API host region — choose EU only for EU data-residency subusers", Options: []core.ConnectionOption{
		{Name: "Global", Value: ""},
		{Name: "EU (data residency)", Value: "eu"},
	}},
	{Name: "events", Type: core.ConnectionTypeMultiSelect, Label: "Events", Placeholder: "Events to fire on (leave empty for all). Recommended: Delivered, Bounced, Opened, Link Clicked, Marked as Spam, Unsubscribed", Options: []core.ConnectionOption{
		{Name: "Processed (accepted by SendGrid)", Value: "processed"},
		{Name: "Delivered", Value: "delivered"},
		{Name: "Deferred (temporarily rejected)", Value: "deferred"},
		{Name: "Bounced", Value: "bounce"},
		{Name: "Blocked", Value: "blocked"},
		{Name: "Dropped (not sent)", Value: "dropped"},
		{Name: "Opened", Value: "open"},
		{Name: "Link Clicked", Value: "click"},
		{Name: "Marked as Spam", Value: "spamreport"},
		{Name: "Unsubscribed", Value: "unsubscribe"},
		{Name: "Group Unsubscribed", Value: "group_unsubscribe"},
		{Name: "Group Resubscribed", Value: "group_resubscribe"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Recipient Email"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Timestamp (Unix)"},
	{Name: "sg_message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "sg_event_id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "sg_machine_open", Type: core.ConnectionTypeString, Label: "Machine Open (Apple MPP)"},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Bounce Type"},
	{Name: "reason", Type: core.ConnectionTypeString, Label: "Reason"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "response", Type: core.ConnectionTypeString, Label: "SMTP Response"},
	{Name: "attempt", Type: core.ConnectionTypeString, Label: "Delivery Attempt"},
	{Name: "category", Type: core.ConnectionTypeString, Label: "Categories"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Clicked URL"},
	{Name: "useragent", Type: core.ConnectionTypeString, Label: "User Agent"},
	{Name: "ip", Type: core.ConnectionTypeString, Label: "IP Address"},
	{Name: "tls", Type: core.ConnectionTypeString, Label: "TLS"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they carry the API key or internal registration settings. The
// launch service injects the event fields (event, email, timestamp, …) at
// fire time.
var configInputs = map[string]bool{
	"api_key":   true,
	"region":    true,
	"events":    true,
	"__node_id": true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing SendGrid webhook trigger")

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
	email := str(data["email"])

	if event == "" {
		event = "event"
	}
	if email != "" {
		return fmt.Sprintf("[SendGrid] %s — %s", event, email)
	}
	return fmt.Sprintf("[SendGrid] %s", event)
}
