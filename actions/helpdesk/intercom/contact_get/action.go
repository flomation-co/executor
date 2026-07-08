package helpdesk_intercom_contact_get

import (
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Get Contact"
	Description  = "Look up a single Intercom contact, either by their Intercom Contact ID or by the External ID you gave them."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+eye"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Intercom access token (Developer Hub → Authentication)", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
	{
		Name:  "select_by",
		Type:  core.ConnectionTypeString,
		Label: "Find By",
		Options: []core.ConnectionOption{
			{Name: "Contact ID", Value: "id"},
			{Name: "External ID", Value: "external_id"},
		},
	},
	{Name: "value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "The Contact ID (or your External ID) to look up", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Contact"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	value, err := intercom.RequiredString("value", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	// External IDs resolve through a dedicated lookup path; the Intercom ID is
	// a plain resource fetch.
	path := "/contacts/" + url.PathEscape(value)
	if intercom.OptionalString("select_by", inputs) == "external_id" {
		path = "/contacts/find_by_external_id/" + url.PathEscape(value)
	}

	obj, err := intercom.GetObject(auth, path, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	label := intercom.StringifyID(obj["id"])
	if name, _ := obj["name"].(string); name != "" {
		label = name
	} else if email, _ := obj["email"].(string); email != "" {
		label = email
	}
	return intercom.ResourceResult(obj, "Retrieved contact "+label), nil
}
