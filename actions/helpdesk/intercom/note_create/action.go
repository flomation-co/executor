package helpdesk_intercom_note_create

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Create Note"
	Description  = "Add a private note to a contact's timeline. Notes are only visible to your teammates — the contact never sees them."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+plus"
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
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Intercom contact the note goes on", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Note", Placeholder: "What your teammates should know about this contact — plain text or simple HTML", Required: true},
	{Name: "admin_id", Type: core.ConnectionTypeString, Label: "Author", Placeholder: "The teammate the note is attributed to"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Note ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Note"},
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
	noteBody, err := intercom.RequiredString("body", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"body": noteBody}
	// admin_id is documented as a JSON integer on notes — send it as a number.
	intercom.SetNumericIDIfPresent(body, inputs, "admin_id", "admin_id")

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/contacts/"+url.PathEscape(contactID)+"/notes", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, "Added a note to contact "+contactID), nil
}
