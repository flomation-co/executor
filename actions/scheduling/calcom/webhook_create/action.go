package scheduling_calcom_webhook_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Create Webhook"
	Description  = "Register a Cal.com webhook that POSTs booking events to a URL you specify."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+plus"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "subscriber_url", Type: core.ConnectionTypeString, Label: "Subscriber URL", Placeholder: "https://example.com/hook", Required: true},
	{Name: "triggers", Type: core.ConnectionTypeMultiSelect, Label: "Events", Required: true, Options: []core.ConnectionOption{
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
	{Name: "active", Type: core.ConnectionTypeBoolean, Label: "Active", Placeholder: "Enable the webhook (default true)"},
	{Name: "secret", Type: core.ConnectionTypeSecret, Label: "Signing Secret", Placeholder: "Used to sign deliveries with HMAC-SHA256 (optional)"},
	{Name: "payload_template", Type: core.ConnectionTypeText, Label: "Payload Template", Placeholder: "Custom {{variable}} payload template (optional)"},
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
	subscriberURL, err := calcom.RequiredString("subscriber_url", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	triggers := calcom.OptionalStringSlice("triggers", inputs)
	if len(triggers) == 0 {
		return calcom.ErrorResult("select at least one event to subscribe to"), nil
	}

	// active defaults to true unless the box is explicitly unchecked.
	active := true
	if conn := core.FindConnection("active", inputs); conn != nil && conn.Boolean() != nil {
		active = *conn.Boolean()
	}

	body := map[string]interface{}{
		"subscriberUrl": subscriberURL,
		"triggers":      triggers,
		"active":        active,
	}
	calcom.SetIfString(body, inputs, "secret", "secret")
	calcom.SetIfString(body, inputs, "payloadTemplate", "payload_template")

	resp, err := calcom.PostResource(token, "/webhooks", calcom.VersionNone, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Created webhook for %s", subscriberURL)), nil
}
