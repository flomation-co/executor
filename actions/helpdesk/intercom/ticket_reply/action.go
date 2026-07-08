package helpdesk_intercom_ticket_reply

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Reply to Ticket"
	Description  = "Add a reply to an Intercom ticket — a customer-visible comment or an internal note as an admin, or a comment on the contact's behalf."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+reply"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

// asAdmin/asContact show only the identity field the chosen reply type needs
// (an empty reply_type defaults to a comment as admin).
var asAdmin = &core.VisibleWhen{Field: "reply_type", Values: []string{"admin_comment", "admin_note", ""}}
var asContact = &core.VisibleWhen{Field: "reply_type", Values: []string{"user_comment"}}

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
	{Name: "ticket_id", Type: core.ConnectionTypeString, Label: "Ticket ID", Placeholder: "The ticket to reply to — from a Create Ticket step or a search", Required: true},
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
	{Name: "admin_id", Type: core.ConnectionTypeString, Label: "Admin", Placeholder: "The teammate the reply is sent as", Visible: asAdmin},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Intercom ID of the contact the reply is from", Visible: asContact},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Message", Placeholder: "What to say — internal notes can include HTML", Required: true},
	{Name: "attachment_urls", Type: core.ConnectionTypeString, Label: "Attachment URLs", Placeholder: "https://…/a.pdf, https://…/b.png — up to 10, comma-separated"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Reply ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Reply"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	ticketID, err := intercom.RequiredString("ticket_id", inputs)
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

	payload := map[string]interface{}{"body": message}
	var summary string
	switch replyType {
	case "admin_comment", "admin_note":
		adminID := intercom.OptionalString("admin_id", inputs)
		if adminID == "" {
			return intercom.ErrorResult("Admin is required when replying as an admin"), nil
		}
		payload["type"] = "admin"
		payload["admin_id"] = adminID
		if replyType == "admin_note" {
			payload["message_type"] = "note"
			summary = fmt.Sprintf("Added an internal note to ticket %s", ticketID)
		} else {
			payload["message_type"] = "comment"
			summary = fmt.Sprintf("Added a comment to ticket %s", ticketID)
		}
	case "user_comment":
		contactID := intercom.OptionalString("contact_id", inputs)
		if contactID == "" {
			return intercom.ErrorResult("Contact ID is required when replying as the contact"), nil
		}
		payload["type"] = "user"
		payload["message_type"] = "comment"
		payload["intercom_user_id"] = contactID
		summary = fmt.Sprintf("Replied to ticket %s as the contact", ticketID)
	default:
		return intercom.ErrorResult("Reply Type must be Comment (as admin), Internal note (as admin), or As contact"), nil
	}
	intercom.SetCSVIfPresent(payload, inputs, "attachment_urls", "attachment_urls")

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/tickets/"+url.PathEscape(ticketID)+"/reply", payload, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, summary), nil
}
