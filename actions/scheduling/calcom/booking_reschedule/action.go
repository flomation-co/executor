package scheduling_calcom_booking_reschedule

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Reschedule Booking"
	Description  = "Move a Cal.com booking to a new start time. Creates a new booking and cancels the original."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+rotate-right"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "booking_uid", Type: core.ConnectionTypeString, Label: "Booking UID", Required: true},
	{Name: "start", Type: core.ConnectionTypeString, Label: "New Start Time", Placeholder: "2026-07-11T09:00:00Z (ISO 8601, UTC)", Required: true},
	{Name: "rescheduling_reason", Type: core.ConnectionTypeString, Label: "Reschedule Reason", Placeholder: "Shared with the attendee (optional)"},
	{Name: "rescheduled_by", Type: core.ConnectionTypeString, Label: "Rescheduled By", Placeholder: "Email of who is rescheduling (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "New Booking UID"},
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
	start, err := calcom.RequiredString("start", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"start": start}
	calcom.SetIfString(body, inputs, "reschedulingReason", "rescheduling_reason")
	calcom.SetIfString(body, inputs, "rescheduledBy", "rescheduled_by")

	resp, err := calcom.PostResource(token, "/bookings/"+url.PathEscape(uid)+"/reschedule", calcom.VersionBookings, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Rescheduled booking %s to %s", uid, start)), nil
}
