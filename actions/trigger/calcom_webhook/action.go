package calcom_webhook

import (
	"fmt"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com Webhook Trigger"
	Description  = "Triggers a flow when a Cal.com event occurs (booking created/rescheduled/cancelled, meeting ended, and more). The webhook subscription is registered with Cal.com automatically."
	Website      = "https://www.flomation.co"
	Icon         = "calcom"
	Date         = "03/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) — used to register the webhook", Required: true},
	{Name: "events", Type: core.ConnectionTypeMultiSelect, Label: "Events", Required: true, Options: []core.ConnectionOption{
		{Name: "Booking Created", Value: "BOOKING_CREATED"},
		{Name: "Booking Rescheduled", Value: "BOOKING_RESCHEDULED"},
		{Name: "Booking Cancelled", Value: "BOOKING_CANCELLED"},
		{Name: "Booking Requested (needs confirmation)", Value: "BOOKING_REQUESTED"},
		{Name: "Booking Rejected", Value: "BOOKING_REJECTED"},
		{Name: "Booking Paid", Value: "BOOKING_PAID"},
		{Name: "Booking No-Show Updated", Value: "BOOKING_NO_SHOW_UPDATED"},
		{Name: "Meeting Started", Value: "MEETING_STARTED"},
		{Name: "Meeting Ended", Value: "MEETING_ENDED"},
		{Name: "Recording Ready", Value: "RECORDING_READY"},
		{Name: "Form Submitted", Value: "FORM_SUBMITTED"},
	}},
	{Name: "event_type_id", Type: core.ConnectionTypeInteger, Label: "Event Type ID", Placeholder: "Only fire for this event type (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "payload", Type: core.ConnectionTypeObject, Label: "Payload"},
	{Name: "attendee_name", Type: core.ConnectionTypeString, Label: "Attendee Name"},
	{Name: "attendee_email", Type: core.ConnectionTypeString, Label: "Attendee Email"},
	{Name: "booking_uid", Type: core.ConnectionTypeString, Label: "Booking UID"},
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Created At"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are trigger configuration fields that must not be echoed as
// outputs — they carry the API key or internal registration settings.
var configInputs = map[string]bool{
	"api_key":       true,
	"events":        true,
	"event_type_id": true,
	"__node_id":     true,
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Cal.com webhook trigger")

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
	name := str(data["attendee_name"])
	email := str(data["attendee_email"])

	if event == "" {
		event = "event"
	}
	who := name
	if who == "" {
		who = email
	}
	if who != "" {
		return fmt.Sprintf("[Cal.com] %s — %s", event, who)
	}
	return fmt.Sprintf("[Cal.com] %s", event)
}
