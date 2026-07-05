package stripe_webhook

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Stripe Webhook Trigger"
	Description  = "Triggers a flow when a Stripe webhook event is received (payment, subscription, invoice…)"
	Website      = "https://www.flomation.co"
	Icon         = "stripe"
	Date         = "05/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "signing_secret", Type: core.ConnectionTypeSecret, Label: "Signing Secret", Placeholder: "whsec_… (from the Stripe webhook endpoint)", Required: true},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "Comma-separated, e.g. payment_intent.succeeded,checkout.session.completed,invoice.paid (blank = all)"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "object_type", Type: core.ConnectionTypeString, Label: "Object Type"},
	{Name: "object_id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID"},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Amount (smallest currency unit)"},
	{Name: "currency", Type: core.ConnectionTypeString, Label: "Currency"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they hold the signing secret and the event filter.
var configInputs = map[string]bool{
	"signing_secret": true,
	"event_filter":   true,
	"__node_id":      true,
}

// Execute echoes the event data Launch resolved from the verified Stripe
// webhook into the flow's outputs. Launch does the signature verification and
// event parsing; this node just surfaces the fields.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Stripe webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	if _, ok := result["content"]; !ok {
		if et, ok := result["event_type"].(string); ok && et != "" {
			result["content"] = "Stripe event: " + et
		}
	}
	return result, nil
}
