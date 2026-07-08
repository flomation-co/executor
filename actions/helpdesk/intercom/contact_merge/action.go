package helpdesk_intercom_contact_merge

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Merge Contacts"
	Description  = "Merge a lead into a user so their conversations and details live on one contact. Intercom only supports merging a lead into a user — not the other way round."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+compress"
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
	{Name: "from_contact_id", Type: core.ConnectionTypeString, Label: "Lead Contact ID", Placeholder: "The lead being merged — their conversations move to the user", Required: true},
	{Name: "into_contact_id", Type: core.ConnectionTypeString, Label: "User Contact ID", Placeholder: "The user who absorbs the lead", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Merged Contact"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	fromID, err := intercom.RequiredString("from_contact_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	intoID, err := intercom.RequiredString("into_contact_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"from": fromID, "into": intoID}
	obj, err := intercom.WriteObject(auth, http.MethodPost, "/contacts/merge", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Merged lead %s into user %s", fromID, intoID)), nil
}
