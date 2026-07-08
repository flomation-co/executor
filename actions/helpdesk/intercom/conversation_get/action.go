package helpdesk_intercom_conversation_get

import (
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Get Conversation"
	Description  = "Fetch a single Intercom conversation with its full message history. Tick Plain Text to strip HTML from the message bodies."
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
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID", Placeholder: "The ID of the conversation, e.g. 123456789", Required: true},
	{Name: "plaintext", Type: core.ConnectionTypeBoolean, Label: "Plain Text", Placeholder: "Return message bodies as plain text instead of HTML"},
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
	q := url.Values{}
	if plain, set := intercom.OptionalBoolSet("plaintext", inputs); set && plain {
		q.Set("display_as", "plaintext")
	}
	obj, err := intercom.GetObject(auth, "/conversations/"+url.PathEscape(id), q)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, "Retrieved conversation "+id), nil
}
