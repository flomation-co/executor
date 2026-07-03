package scheduling_calcom_booking_decline

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Decline Booking"
	Description  = "Reject a Cal.com booking that requires confirmation, optionally with a reason."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+circle-xmark"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "booking_uid", Type: core.ConnectionTypeString, Label: "Booking UID", Required: true},
	{Name: "reason", Type: core.ConnectionTypeString, Label: "Reason", Placeholder: "Shared with the attendee (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Booking UID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Booking"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	uid, err := calcom.RequiredString("booking_uid", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	calcom.SetIfString(body, inputs, "reason", "reason")

	resp, err := calcom.PostResource(token, "/bookings/"+url.PathEscape(uid)+"/decline", calcom.VersionBookings, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Declined booking %s", uid)), nil
}
