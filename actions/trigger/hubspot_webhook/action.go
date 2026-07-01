package hubspot_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "HubSpot Webhook Trigger"
	Description  = "Triggers a flow when a HubSpot webhook event is received (contact/company/deal/ticket creation, deletion, or property change)."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot"
	Date         = "30/06/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret", Placeholder: "Client secret for X-HubSpot-Signature validation"},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "Comma-separated: contact.creation,deal.propertyChange,ticket.creation"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Subscription Type"},
	{Name: "object_id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "property_name", Type: core.ConnectionTypeString, Label: "Property Name"},
	{Name: "property_value", Type: core.ConnectionTypeString, Label: "Property Value"},
	{Name: "change_source", Type: core.ConnectionTypeString, Label: "Change Source"},
	{Name: "portal_id", Type: core.ConnectionTypeString, Label: "Portal ID"},
	{Name: "occurred_at", Type: core.ConnectionTypeString, Label: "Occurred At"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they hold secrets or internal filter settings.
var configInputs = map[string]bool{
	"app_secret":   true,
	"event_filter": true,
	"__node_id":    true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing HubSpot webhook trigger")

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
	objectID := str(data["object_id"])
	prop := str(data["property_name"])
	val := str(data["property_value"])

	if eventType == "" {
		return "[HubSpot Event] received"
	}
	if prop != "" {
		return fmt.Sprintf("[HubSpot] %s on object %s: %s = %s", eventType, objectID, prop, val)
	}
	return fmt.Sprintf("[HubSpot] %s on object %s", eventType, objectID)
}
