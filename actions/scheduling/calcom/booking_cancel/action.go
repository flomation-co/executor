package scheduling_calcom_booking_cancel

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Cancel Booking"
	Description  = "Cancel a Cal.com booking, optionally with a reason."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+ban"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "booking_uid", Type: core.ConnectionTypeString, Label: "Booking UID", Required: true},
	{Name: "cancellation_reason", Type: core.ConnectionTypeString, Label: "Cancellation Reason", Placeholder: "Shared with the attendee (optional)"},
	{Name: "cancel_subsequent", Type: core.ConnectionTypeBoolean, Label: "Cancel Subsequent (recurring)", Placeholder: "Cancel all following bookings in a recurring series"},
	{Name: "seat_uid", Type: core.ConnectionTypeString, Label: "Seat UID", Placeholder: "Required to cancel a single seat of a seated event (optional)"},
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
	calcom.SetIfString(body, inputs, "cancellationReason", "cancellation_reason")
	calcom.SetIfString(body, inputs, "seatUid", "seat_uid")
	if calcom.OptionalBool("cancel_subsequent", inputs) {
		body["cancelSubsequentBookings"] = true
	}

	resp, err := calcom.PostResource(token, "/bookings/"+url.PathEscape(uid)+"/cancel", calcom.VersionBookings, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Cancelled booking %s", uid)), nil
}
