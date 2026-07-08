package helpdesk_intercom_contact_archive

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Archive Contact"
	Description  = "Archive a contact to hide them from your Intercom workspace without deleting their data. Bring them back any time with Unarchive Contact."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+box-archive"
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
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Intercom ID of the contact to archive", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	contactID, err := intercom.RequiredString("contact_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	obj, err := intercom.WriteObject(auth, http.MethodPost, "/contacts/"+url.PathEscape(contactID)+"/archive", nil, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, "Archived contact "+contactID), nil
}
