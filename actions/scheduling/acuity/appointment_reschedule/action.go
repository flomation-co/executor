package scheduling_acuity_appointment_reschedule

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Reschedule Appointment"
	Description  = "Move an Acuity appointment to a new date and time (and optionally a different calendar)."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+rotate-right"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "appointment_id", Type: core.ConnectionTypeInteger, Label: "Appointment ID", Required: true},
	{Name: "datetime", Type: core.ConnectionTypeString, Label: "New Date & Time", Placeholder: "2026-07-11T09:00:00-0700 (ISO 8601)", Required: true},
	{Name: "calendar_id", Type: core.ConnectionTypeInteger, Label: "Calendar ID", Placeholder: "Move to a different calendar (optional)"},
	{Name: "admin", Type: core.ConnectionTypeBoolean, Label: "Admin Reschedule", Placeholder: "Bypass availability checks"},
	{Name: "no_email", Type: core.ConnectionTypeBoolean, Label: "Suppress Emails"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Appointment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Appointment"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	userID, apiKey, err := acuity.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := acuity.RequiredInt("appointment_id", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	datetime, err := acuity.RequiredString("datetime", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"datetime": datetime}
	if calID, ok := acuity.OptionalInt("calendar_id", inputs); ok {
		body["calendarID"] = calID
	}

	q := url.Values{}
	if acuity.OptionalBool("admin", inputs) {
		q.Set("admin", "true")
	}
	if acuity.OptionalBool("no_email", inputs) {
		q.Set("noEmail", "true")
	}

	resp, err := acuity.PutObject(userID, apiKey, fmt.Sprintf("/appointments/%d/reschedule", id), q, body)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ResourceResult(resp, fmt.Sprintf("Rescheduled appointment %d to %s", id, datetime)), nil
}
