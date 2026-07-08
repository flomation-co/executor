package helpdesk_intercom_tag_delete

import (
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Delete Tag"
	Description  = "Delete a tag from Intercom. A tag still applied to contacts, companies, or conversations can't be deleted — remove it from them first."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+trash"
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
	{Name: "tag_id", Type: core.ConnectionTypeString, Label: "Tag", Placeholder: "The tag to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Tag ID"},
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
	id, err := intercom.RequiredString("tag_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	// Intercom refuses to delete a tag that is still applied to anything; its
	// error explains where, and CheckResponse surfaces that message verbatim.
	if err := intercom.DeleteResource(auth, "/tags/"+url.PathEscape(id)); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.SuccessResult(id, nil, "Deleted tag "+id), nil
}
