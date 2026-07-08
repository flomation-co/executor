package helpdesk_intercom_conversation_tag_remove

import (
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Remove Tag from Conversation"
	Description  = "Remove a tag from an Intercom conversation."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+minus"
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
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID", Placeholder: "The ID of the conversation to untag", Required: true},
	{Name: "tag_id", Type: core.ConnectionTypeString, Label: "Tag", Placeholder: "The tag to remove", Required: true},
	{Name: "admin_id", Type: core.ConnectionTypeString, Label: "Admin", Placeholder: "The teammate removing the tag", Required: true},
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
	id, err := intercom.RequiredString("conversation_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	tagID, err := intercom.RequiredString("tag_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	adminID, err := intercom.RequiredString("admin_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	// Intercom's untag is a DELETE that REQUIRES a JSON body carrying the
	// acting admin.
	path := "/conversations/" + url.PathEscape(id) + "/tags/" + url.PathEscape(tagID)
	if err := intercom.DeleteWithBody(auth, path, map[string]interface{}{"admin_id": adminID}); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.SuccessResult(tagID, nil, "Removed tag from conversation "+id), nil
}
