package scheduling_calcom_booking_mark_absent

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Mark Booking Absent"
	Description  = "Mark the host and/or specific attendees of a Cal.com booking as no-shows."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+xmark"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "booking_uid", Type: core.ConnectionTypeString, Label: "Booking UID", Required: true},
	{Name: "host_absent", Type: core.ConnectionTypeBoolean, Label: "Host Absent", Placeholder: "Mark the host as a no-show"},
	{Name: "attendees_json", Type: core.ConnectionTypeObject, Label: "Attendees (JSON array, advanced)", Placeholder: `[{"email":"a@b.com","absent":true}]`},
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
	calcom.SetIfBoolPresent(body, inputs, "host", "host_absent")
	if attendees, err := calcom.ParseJSONArray("attendees_json", inputs); err != nil {
		return calcom.ErrorResult(err.Error()), nil
	} else if attendees != nil {
		body["attendees"] = attendees
	}

	resp, err := calcom.PostResource(token, "/bookings/"+url.PathEscape(uid)+"/mark-absent", calcom.VersionBookings, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Marked no-shows on booking %s", uid)), nil
}
