package helpdesk_intercom_message_send

import (
	"fmt"
	"net/http"
	"strconv"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Send Message"
	Description  = "Send an outbound in-app or email message from an admin to a contact. Emails also need a Subject; in-app messages just need a Body."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+paper-plane"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var emailOnly = &core.VisibleWhen{Field: "message_type", Values: []string{"email"}}

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
		Name:  "message_type",
		Type:  core.ConnectionTypeString,
		Label: "Message Type",
		Options: []core.ConnectionOption{
			{Name: "In-app message", Value: "in_app"},
			{Name: "Email", Value: "email"},
		},
	},
	{Name: "from_admin_id", Type: core.ConnectionTypeString, Label: "From (Admin)", Placeholder: "The teammate the message is sent from", Required: true},
	{Name: "to_contact_id", Type: core.ConnectionTypeString, Label: "To (Contact ID)", Placeholder: "The Intercom contact ID of the person to message", Required: true},
	{
		Name:  "to_type",
		Type:  core.ConnectionTypeString,
		Label: "Recipient Type",
		Options: []core.ConnectionOption{
			{Name: "User", Value: "user"},
			{Name: "Lead", Value: "lead"},
		},
	},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "The email subject line", Visible: emailOnly},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Body", Placeholder: "The message to send — emails may use HTML", Required: true},
	{
		Name:    "template",
		Type:    core.ConnectionTypeString,
		Label:   "Email Template",
		Visible: emailOnly,
		Options: []core.ConnectionOption{
			{Name: "Plain", Value: "plain"},
			{Name: "Personal", Value: "personal"},
		},
	},
	{Name: "create_conversation_without_contact_reply", Type: core.ConnectionTypeBoolean, Label: "Open Conversation Immediately", Placeholder: "Create the conversation straight away instead of waiting for the contact to reply"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Message ID"},
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

	fromID, err := intercom.RequiredString("from_admin_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	toID, err := intercom.RequiredString("to_contact_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	bodyText, err := intercom.RequiredString("body", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	messageType := intercom.OptionalString("message_type", inputs)
	if messageType == "" {
		messageType = "in_app"
	}
	if messageType != "in_app" && messageType != "email" {
		return intercom.ErrorResult("Message Type must be In-app message or Email"), nil
	}
	toType := intercom.OptionalString("to_type", inputs)
	if toType == "" {
		toType = "user"
	}
	if toType != "user" && toType != "lead" {
		return intercom.ErrorResult("Recipient Type must be User or Lead"), nil
	}

	// from.id is typed as a JSON integer by Intercom — a string admin ID is
	// rejected, so a numeric value is converted (non-numeric passes through for
	// Intercom to surface its own error).
	from := map[string]interface{}{"type": "admin"}
	if n, err := strconv.ParseInt(fromID, 10, 64); err == nil {
		from["id"] = n
	} else {
		from["id"] = fromID
	}

	body := map[string]interface{}{
		"message_type": messageType,
		"from":         from,
		"to":           map[string]interface{}{"type": toType, "id": toID},
		"body":         bodyText,
	}
	if messageType == "email" {
		subject := intercom.OptionalString("subject", inputs)
		if subject == "" {
			return intercom.ErrorResult("a Subject is required for email messages"), nil
		}
		body["subject"] = subject
		template := intercom.OptionalString("template", inputs)
		if template == "" {
			template = "plain"
		}
		if template != "plain" && template != "personal" {
			return intercom.ErrorResult("Email Template must be Plain or Personal"), nil
		}
		body["template"] = template
	}
	intercom.SetBoolIfSet(body, inputs, "create_conversation_without_contact_reply", "create_conversation_without_contact_reply")

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/messages", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	kind := "in-app"
	if messageType == "email" {
		kind = "email"
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Sent %s message to %s %s", kind, toType, toID)), nil
}
