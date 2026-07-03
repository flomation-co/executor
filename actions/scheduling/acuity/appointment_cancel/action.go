package scheduling_acuity_appointment_cancel

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Cancel Appointment"
	Description  = "Cancel an Acuity appointment, optionally with a note."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+ban"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "appointment_id", Type: core.ConnectionTypeInteger, Label: "Appointment ID", Required: true},
	{Name: "cancel_note", Type: core.ConnectionTypeText, Label: "Cancellation Note", Placeholder: "Reason shared internally (optional)"},
	{Name: "admin", Type: core.ConnectionTypeBoolean, Label: "Admin Cancel", Placeholder: "Bypass cancellation restrictions"},
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

	body := map[string]interface{}{}
	acuity.SetIfString(body, inputs, "cancelNote", "cancel_note")

	q := url.Values{}
	if acuity.OptionalBool("admin", inputs) {
		q.Set("admin", "true")
	}
	if acuity.OptionalBool("no_email", inputs) {
		q.Set("noEmail", "true")
	}

	resp, err := acuity.PutObject(userID, apiKey, fmt.Sprintf("/appointments/%d/cancel", id), q, body)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ResourceResult(resp, fmt.Sprintf("Cancelled appointment %d", id)), nil
}
