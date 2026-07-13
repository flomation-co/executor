package xero_webhook

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Xero Webhook Trigger"
	Description  = "Triggers a flow when a Xero record changes (contact or invoice created/updated)"
	Website      = "https://www.flomation.co"
	Icon         = "xero"
	Date         = "10/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "signing_key", Type: core.ConnectionTypeSecret, Label: "Webhook Signing Key", Placeholder: "From the Xero app's webhook settings", Required: true},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "Comma-separated, e.g. CONTACT.UPDATE,INVOICE.CREATE or CONTACT (blank = all)"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "event_category", Type: core.ConnectionTypeString, Label: "Event Category"},
	{Name: "operation", Type: core.ConnectionTypeString, Label: "Operation"},
	{Name: "resource_id", Type: core.ConnectionTypeString, Label: "Resource ID"},
	{Name: "tenant_id", Type: core.ConnectionTypeString, Label: "Organisation (Tenant ID)"},
	{Name: "resource_url", Type: core.ConnectionTypeString, Label: "Resource URL"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are provided by the flow author; they are not echoed to outputs.
var configInputs = map[string]bool{
	"signing_key":  true,
	"event_filter": true,
	"__node_id":    true,
}

// Execute echoes the event data Launch resolved from the verified Xero webhook
// into the flow's outputs. Launch does the signature verification and event
// parsing; this node just surfaces the fields.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	if _, ok := result["content"]; !ok {
		if et, ok := result["event_type"].(string); ok && et != "" {
			result["content"] = "Xero event: " + et
		}
	}
	return result, nil
}
