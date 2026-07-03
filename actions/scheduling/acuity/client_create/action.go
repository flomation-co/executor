package scheduling_acuity_client_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Create Client"
	Description  = "Add a new client to your Acuity address book."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+user-plus"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Required: true},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "client@example.com (optional)"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "Client phone (optional)"},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes", Placeholder: "Internal notes (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Client ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Client"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	userID, apiKey, err := acuity.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	firstName, err := acuity.RequiredString("first_name", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	lastName, err := acuity.RequiredString("last_name", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"firstName": firstName, "lastName": lastName}
	acuity.SetIfString(body, inputs, "email", "email")
	acuity.SetIfString(body, inputs, "phone", "phone")
	acuity.SetIfString(body, inputs, "notes", "notes")

	resp, err := acuity.PostObject(userID, apiKey, "/clients", nil, body)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ResourceResult(resp, fmt.Sprintf("Created client %s %s", firstName, lastName)), nil
}
