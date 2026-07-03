package scheduling_acuity_client_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Delete Client"
	Description  = "Remove a client from your Acuity address book. The client is identified by first name, last name and phone."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+trash"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Required: true},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Required: true},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Required: true},
}

var Outputs = [...]core.Connection{
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
	phone, err := acuity.RequiredString("phone", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	q.Set("firstName", firstName)
	q.Set("lastName", lastName)
	q.Set("phone", phone)

	if err := acuity.DeleteResource(userID, apiKey, "/clients", q); err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted client %s %s", firstName, lastName),
		"success":     true,
		"error":       "",
	}, nil
}
