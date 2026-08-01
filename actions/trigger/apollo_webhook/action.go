package apollo_webhook

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Apollo Webhook Trigger"
	Description  = "Triggers a flow when an Apollo.io webhook fires (contact/account created, sequence engagement…)"
	Website      = "https://www.flomation.co"
	Icon         = "apollo"
	Date         = "01/08/2026"
	Type         = core.ActionTypeTrigger
)

// Apollo publishes NO webhook signature or signing secret, so authenticity
// cannot be proven cryptographically. Instead the flow author sets a
// `webhook_secret` which Launch requires as a token on the inbound request
// (query param `?secret=` or the X-Flomation-Webhook-Secret header) and
// compares in constant time. The generic /webhook/:id route already keys on an
// unguessable trigger id; the secret is defence-in-depth on top of that.
var Inputs = [...]core.Connection{
	{Name: "webhook_secret", Type: core.ConnectionTypeSecret, Label: "Webhook Secret", Placeholder: "A secret token you choose; append it to the webhook URL as ?secret=…", Required: true},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "Comma-separated event types, e.g. contact.created,account.created (blank = all)"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they hold the shared secret and the event filter.
var configInputs = map[string]bool{
	"webhook_secret": true,
	"event_filter":   true,
	"__node_id":      true,
}

// Execute echoes the event data Launch resolved from the verified Apollo webhook
// into the flow's outputs. Launch does the secret-token check and JSON parsing;
// this node just surfaces the fields.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Apollo webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	if _, ok := result["content"]; !ok {
		if et, ok := result["event_type"].(string); ok && et != "" {
			result["content"] = "Apollo event: " + et
		}
	}
	return result, nil
}
