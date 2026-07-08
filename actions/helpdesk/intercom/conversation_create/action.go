package helpdesk_intercom_conversation_create

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Create Conversation"
	Description  = "Start a new conversation in Intercom on behalf of a contact — the message appears as if the user or lead wrote it themselves."
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
	{
		Name:  "from_type",
		Type:  core.ConnectionTypeString,
		Label: "From",
		Options: []core.ConnectionOption{
			{Name: "User", Value: "user"},
			{Name: "Lead", Value: "lead"},
		},
	},
	{Name: "from_contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Intercom ID of the contact starting the conversation", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Message", Placeholder: "The first message of the conversation", Required: true},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "An optional subject line"},
	{Name: "attachment_urls", Type: core.ConnectionTypeString, Label: "Attachment URLs", Placeholder: "Comma-separated file URLs to attach (up to 10)"},
	{Name: "created_at", Type: core.ConnectionTypeDateTime, Label: "Created At", Placeholder: "Backdate the conversation, e.g. 2026-07-08T09:00:00Z"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Conversation ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Message"},
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
	message, err := intercom.RequiredString("body", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	fromType := intercom.OptionalString("from_type", inputs)
	if fromType == "" {
		fromType = "user"
	}

	body := map[string]interface{}{
		"from": map[string]interface{}{"type": fromType, "id": fromID},
		"body": message,
	}
	intercom.SetIfPresent(body, inputs, "subject", "subject")
	if urls := intercom.SplitCSV(intercom.OptionalString("attachment_urls", inputs)); urls != nil {
		if len(urls) > 10 {
			return intercom.ErrorResult("Intercom accepts at most 10 attachment URLs per message"), nil
		}
		body["attachment_urls"] = urls
	}
	if err := intercom.SetUnixIfPresent(body, inputs, "created_at", "created_at"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/conversations", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	// Intercom echoes the created MESSAGE, not the conversation — its
	// conversation_id is the handle downstream steps chain on, so prefer it
	// as the id output.
	id := intercom.StringifyID(obj["conversation_id"])
	if id == "" {
		id = intercom.StringifyID(obj["id"])
	}
	return intercom.SuccessResult(id, obj, fmt.Sprintf("Started conversation from %s %s", fromType, fromID)), nil
}
