package scheduling_calcom_webhook_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Update Webhook"
	Description  = "Update a Cal.com webhook's URL, events, active state or secret. Only supplied fields change."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+pencil"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "webhook_id", Type: core.ConnectionTypeString, Label: "Webhook ID", Required: true},
	{Name: "subscriber_url", Type: core.ConnectionTypeString, Label: "Subscriber URL", Placeholder: "New URL (optional)"},
	{Name: "triggers", Type: core.ConnectionTypeMultiSelect, Label: "Events", Placeholder: "Replace the subscribed events (optional)", Options: []core.ConnectionOption{
		{Name: "Booking Created", Value: "BOOKING_CREATED"},
		{Name: "Booking Rescheduled", Value: "BOOKING_RESCHEDULED"},
		{Name: "Booking Cancelled", Value: "BOOKING_CANCELLED"},
		{Name: "Booking Requested", Value: "BOOKING_REQUESTED"},
		{Name: "Booking Rejected", Value: "BOOKING_REJECTED"},
		{Name: "Booking Paid", Value: "BOOKING_PAID"},
		{Name: "Booking No-Show Updated", Value: "BOOKING_NO_SHOW_UPDATED"},
		{Name: "Meeting Started", Value: "MEETING_STARTED"},
		{Name: "Meeting Ended", Value: "MEETING_ENDED"},
		{Name: "Recording Ready", Value: "RECORDING_READY"},
		{Name: "Form Submitted", Value: "FORM_SUBMITTED"},
	}},
	{Name: "active", Type: core.ConnectionTypeBoolean, Label: "Active"},
	{Name: "secret", Type: core.ConnectionTypeSecret, Label: "Signing Secret", Placeholder: "New signing secret (optional)"},
	{Name: "payload_template", Type: core.ConnectionTypeText, Label: "Payload Template", Placeholder: "New payload template (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Webhook ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Webhook"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := calcom.RequiredString("webhook_id", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	calcom.SetIfString(body, inputs, "subscriberUrl", "subscriber_url")
	if triggers := calcom.OptionalStringSlice("triggers", inputs); len(triggers) > 0 {
		body["triggers"] = triggers
	}
	calcom.SetIfBoolPresent(body, inputs, "active", "active")
	calcom.SetIfString(body, inputs, "secret", "secret")
	calcom.SetIfString(body, inputs, "payloadTemplate", "payload_template")
	if len(body) == 0 {
		return calcom.ErrorResult("no fields to update: supply at least one field"), nil
	}

	resp, err := calcom.PatchResource(token, "/webhooks/"+url.PathEscape(id), calcom.VersionNone, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Updated webhook %s", id)), nil
}
