package scheduling_acuity_appointment_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Update Appointment"
	Description  = "Update a client's details or notes on an Acuity appointment. Only supplied fields change."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+pencil"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "appointment_id", Type: core.ConnectionTypeInteger, Label: "Appointment ID", Required: true},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "New first name (optional)"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "New last name (optional)"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "New email (optional)"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "New phone (optional)"},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes", Placeholder: "New notes (optional)"},
	{Name: "fields_json", Type: core.ConnectionTypeObject, Label: "Intake Form Fields (JSON array, advanced)", Placeholder: `[{"id":1,"value":"answer"}]`},
	{Name: "admin", Type: core.ConnectionTypeBoolean, Label: "Admin Update", Placeholder: "Bypass restrictions"},
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
	acuity.SetIfString(body, inputs, "firstName", "first_name")
	acuity.SetIfString(body, inputs, "lastName", "last_name")
	acuity.SetIfString(body, inputs, "email", "email")
	acuity.SetIfString(body, inputs, "phone", "phone")
	acuity.SetIfString(body, inputs, "notes", "notes")
	if fields, err := acuity.ParseJSONArray("fields_json", inputs); err != nil {
		return acuity.ErrorResult(err.Error()), nil
	} else if fields != nil {
		body["fields"] = fields
	}
	if len(body) == 0 {
		return acuity.ErrorResult("no fields to update: supply at least one field"), nil
	}

	q := url.Values{}
	if acuity.OptionalBool("admin", inputs) {
		q.Set("admin", "true")
	}
	if acuity.OptionalBool("no_email", inputs) {
		q.Set("noEmail", "true")
	}

	resp, err := acuity.PutObject(userID, apiKey, fmt.Sprintf("/appointments/%d", id), q, body)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ResourceResult(resp, fmt.Sprintf("Updated appointment %d", id)), nil
}
