package helpdesk_intercom_conversation_reply

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Reply to Conversation"
	Description  = "Add a reply to an Intercom conversation — a public comment or internal note as an admin, or a reply on the contact's behalf."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+reply"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var adminReply = &core.VisibleWhen{Field: "reply_type", Values: []string{"admin_comment", "admin_note", ""}}
var contactReply = &core.VisibleWhen{Field: "reply_type", Values: []string{"user_comment"}}

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
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID", Placeholder: "The ID of the conversation to reply to", Required: true},
	{
		Name:  "reply_type",
		Type:  core.ConnectionTypeString,
		Label: "Reply Type",
		Options: []core.ConnectionOption{
			{Name: "Comment (as admin)", Value: "admin_comment"},
			{Name: "Internal note (as admin)", Value: "admin_note"},
			{Name: "As contact", Value: "user_comment"},
		},
	},
	{Name: "admin_id", Type: core.ConnectionTypeString, Label: "Admin", Placeholder: "The teammate writing the reply", Visible: adminReply},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Intercom ID of the contact to reply as", Visible: contactReply},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Message", Placeholder: "The reply text (internal notes may include HTML)", Required: true},
	{Name: "attachment_urls", Type: core.ConnectionTypeString, Label: "Attachment URLs", Placeholder: "Comma-separated file URLs to attach (up to 10)"},
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
	message, err := intercom.RequiredString("body", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	replyType := intercom.OptionalString("reply_type", inputs)
	if replyType == "" {
		replyType = "admin_comment"
	}

	body := map[string]interface{}{"body": message}
	var summary string
	switch replyType {
	case "admin_comment", "admin_note":
		adminID := intercom.OptionalString("admin_id", inputs)
		if adminID == "" {
			return intercom.ErrorResult("pick the Admin writing this reply"), nil
		}
		body["type"] = "admin"
		body["admin_id"] = adminID
		if replyType == "admin_note" {
			body["message_type"] = "note"
			summary = "Added internal note to conversation " + id
		} else {
			body["message_type"] = "comment"
			summary = "Replied to conversation " + id
		}
	case "user_comment":
		contactID := intercom.OptionalString("contact_id", inputs)
		if contactID == "" {
			return intercom.ErrorResult("provide the Contact ID of the contact to reply as"), nil
		}
		body["type"] = "user"
		body["message_type"] = "comment"
		body["intercom_user_id"] = contactID
		summary = "Replied to conversation " + id + " as the contact"
	default:
		return intercom.ErrorResult("Reply Type must be Comment (as admin), Internal note (as admin), or As contact"), nil
	}
	intercom.SetCSVIfPresent(body, inputs, "attachment_urls", "attachment_urls")

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/conversations/"+url.PathEscape(id)+"/reply", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, summary), nil
}
