package acuity_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity Scheduling Trigger"
	Description  = "Triggers a flow when an Acuity Scheduling event occurs (appointment scheduled/rescheduled/canceled/changed, or order completed). The webhook subscription is registered with Acuity automatically."
	Website      = "https://www.flomation.co"
	Icon         = "acuity"
	Date         = "04/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Placeholder: "Used to register the webhook", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Used to register the webhook and verify signatures", Required: true},
	{Name: "events", Type: core.ConnectionTypeMultiSelect, Label: "Events", Required: true, Options: []core.ConnectionOption{
		{Name: "Appointment Scheduled", Value: "appointment.scheduled"},
		{Name: "Appointment Rescheduled", Value: "appointment.rescheduled"},
		{Name: "Appointment Canceled", Value: "appointment.canceled"},
		{Name: "Appointment Changed (any change)", Value: "appointment.changed"},
		{Name: "Order Completed", Value: "order.completed"},
	}},
	{Name: "resolve_data", Type: core.ConnectionTypeBoolean, Label: "Resolve Full Object", Placeholder: "Fetch the full appointment/order (Acuity only sends its ID). Default on."},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "object_id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "calendar_id", Type: core.ConnectionTypeString, Label: "Calendar ID"},
	{Name: "appointment_type_id", Type: core.ConnectionTypeString, Label: "Appointment Type ID"},
	{Name: "appointment", Type: core.ConnectionTypeObject, Label: "Resolved Object"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they carry the credentials or internal registration settings.
var configInputs = map[string]bool{
	"user_id":      true,
	"api_key":      true,
	"events":       true,
	"resolve_data": true,
	"__node_id":    true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Acuity Scheduling trigger")

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
	id := str(data["object_id"])
	if event == "" {
		event = "event"
	}
	if id != "" {
		return fmt.Sprintf("[Acuity] %s — #%s", event, id)
	}
	return fmt.Sprintf("[Acuity] %s", event)
}
