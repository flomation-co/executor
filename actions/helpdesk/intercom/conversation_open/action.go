package helpdesk_intercom_conversation_open

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Open Conversation"
	Description  = "Reopen a closed or snoozed Intercom conversation."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+rotate-right"
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
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID", Placeholder: "The ID of the conversation to reopen", Required: true},
	{Name: "admin_id", Type: core.ConnectionTypeString, Label: "Admin", Placeholder: "The teammate reopening the conversation", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Conversation ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Conversation"},
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
	adminID, err := intercom.RequiredString("admin_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{
		"message_type": "open",
		"admin_id":     adminID,
	}

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/conversations/"+url.PathEscape(id)+"/parts", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, "Opened conversation "+id), nil
}
