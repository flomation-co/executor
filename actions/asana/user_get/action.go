package asana_user_get

import (
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get User"
	Description  = "Fetch a single Asana user by ID (or 'me' for the authenticated user)."
	Website      = "https://www.flomation.co"
	Icon         = "asana+user"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User", Placeholder: "The user ID, or 'me' for yourself", Required: true},
	{Name: "opt_fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. name,email"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := asana.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	userID, err := asana.RequiredString("user_id", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	q := url.Values{}
	asana.SetOptFields(q, inputs, "opt_fields")
	obj, err := asana.GetObject(auth, "/users/"+url.PathEscape(userID), q)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ResourceResult(obj, "Retrieved user "+userID), nil
}
