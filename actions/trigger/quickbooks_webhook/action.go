package quickbooks_webhook

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "QuickBooks Webhook Trigger"
	Description  = "Triggers a flow when a QuickBooks Online entity changes (customer, invoice, payment…)"
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks"
	Date         = "10/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "verifier_token", Type: core.ConnectionTypeSecret, Label: "Verifier Token", Placeholder: "From the Intuit app's webhooks settings", Required: true},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "Comma-separated, e.g. Customer.Create,Invoice.Update or Customer (blank = all)"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "entity", Type: core.ConnectionTypeString, Label: "Entity"},
	{Name: "entity_id", Type: core.ConnectionTypeString, Label: "Entity ID"},
	{Name: "operation", Type: core.ConnectionTypeString, Label: "Operation"},
	{Name: "realm_id", Type: core.ConnectionTypeString, Label: "Company (Realm ID)"},
	{Name: "last_updated", Type: core.ConnectionTypeString, Label: "Last Updated"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are provided by the flow author; they are not echoed to outputs.
var configInputs = map[string]bool{
	"verifier_token": true,
	"event_filter":   true,
	"__node_id":      true,
}

// Execute echoes the event data Launch resolved from the verified QuickBooks
// webhook into the flow's outputs. Launch does the signature verification and
// event parsing; this node just surfaces the fields.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	if _, ok := result["content"]; !ok {
		if et, ok := result["event_type"].(string); ok && et != "" {
			result["content"] = "QuickBooks event: " + et
		}
	}
	return result, nil
}
