package freshsales_webhook

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Freshsales Webhook Trigger"
	Description  = "Triggers a flow when a Freshsales workflow webhook fires (contact, account or deal changes)."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks"
	Date         = "04/09/2026"
	Type         = core.ActionTypeTrigger
)

// Freshsales webhooks are configured in the customer's Workflows UI and carry
// NO signature, signing secret or HMAC — so authenticity cannot be proven
// cryptographically, exactly as with Apollo.
//
// It does give the admin somewhere sensible to put a shared secret, which
// Apollo does not: a webhook action offers Token authentication and custom
// headers. Launch therefore accepts the secret three ways, in order of how
// pleasant they are to configure:
//
//	Authorization: Token <secret>          — Freshsales' own Token auth field
//	X-Flomation-Webhook-Secret: <secret>   — its custom-headers field
//	?secret=<secret>                       — appended to the URL, last resort
//
// All three are compared in constant time. The generic /webhook/:id route
// already keys on an unguessable trigger id; the secret is defence in depth.
var Inputs = [...]core.Connection{
	{Name: "webhook_secret", Type: core.ConnectionTypeSecret, Label: "Webhook Secret", Placeholder: "A token you choose — paste it into the webhook's Token auth field in Freshsales", Required: true},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "Comma-separated event names, e.g. contact.created,deal.won (blank = all)"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "entity_type", Type: core.ConnectionTypeString, Label: "Entity Type"},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Freshsales Account ID"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must never be echoed as
// outputs — the shared secret above all.
var configInputs = map[string]bool{
	"webhook_secret": true,
	"event_filter":   true,
	"__node_id":      true,
}

// Execute echoes the event data Launch resolved from the verified webhook into
// the flow's outputs. Launch does the secret check and the JSON parsing; this
// node only surfaces the fields.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Freshsales webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	if _, ok := result["content"]; !ok {
		if et, ok := result["event_type"].(string); ok && et != "" {
			result["content"] = "Freshsales event: " + et
		}
	}
	return result, nil
}
