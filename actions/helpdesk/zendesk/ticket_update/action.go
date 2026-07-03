package helpdesk_zendesk_ticket_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Update Ticket"
	Description  = "Update a Zendesk ticket. Add a public reply or an internal note, change status/priority/assignee, and set any other ticket field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+pencil"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{Name: "ticket_id", Type: core.ConnectionTypeString, Label: "Ticket ID", Placeholder: "35436", Required: true},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject"},
	{Name: "public_reply", Type: core.ConnectionTypeText, Label: "Public Reply", Placeholder: "A reply added as a public comment on the ticket"},
	{Name: "internal_note", Type: core.ConnectionTypeText, Label: "Internal Note", Placeholder: "A private note (accepts HTML) visible only to agents"},
	{
		Name:  "type",
		Type:  core.ConnectionTypeString,
		Label: "Type",
		Options: []core.ConnectionOption{
			{Name: "Question", Value: "question"},
			{Name: "Incident", Value: "incident"},
			{Name: "Problem", Value: "problem"},
			{Name: "Task", Value: "task"},
		},
	},
	{
		Name:  "priority",
		Type:  core.ConnectionTypeString,
		Label: "Priority",
		Options: []core.ConnectionOption{
			{Name: "Urgent", Value: "urgent"},
			{Name: "High", Value: "high"},
			{Name: "Normal", Value: "normal"},
			{Name: "Low", Value: "low"},
		},
	},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
		Options: []core.ConnectionOption{
			{Name: "New", Value: "new"},
			{Name: "Open", Value: "open"},
			{Name: "Pending", Value: "pending"},
			{Name: "On-Hold", Value: "hold"},
			{Name: "Solved", Value: "solved"},
			{Name: "Closed", Value: "closed"},
		},
	},
	{Name: "recipient", Type: core.ConnectionTypeString, Label: "Recipient", Placeholder: "The original recipient email address of the ticket"},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "The group this ticket is assigned to (ID)"},
	{Name: "assignee_id", Type: core.ConnectionTypeString, Label: "Assignee ID", Placeholder: "Agent user ID to assign the ticket to"},
	{Name: "assignee_email", Type: core.ConnectionTypeString, Label: "Assignee Email", Placeholder: "Email of the agent to assign the ticket to"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "External ID", Placeholder: "An ID linking this ticket to a record in another system"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "vip, refund (comma-separated) — replaces the ticket's tags"},
	{Name: "custom_fields", Type: core.ConnectionTypeObject, Label: "Custom Fields (JSON)", Placeholder: `[{"id":123,"value":"blue"}]`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"collaborator_ids":[1,2]}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ticket ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Ticket"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	subdomain, auth, err := zendesk.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := zendesk.RequiredString("ticket_id", inputs)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	ticket := map[string]interface{}{}
	zendesk.SetIfPresent(ticket, inputs, "subject", "subject")
	zendesk.SetIfPresent(ticket, inputs, "type", "type")
	zendesk.SetIfPresent(ticket, inputs, "priority", "priority")
	zendesk.SetIfPresent(ticket, inputs, "status", "status")
	zendesk.SetIfPresent(ticket, inputs, "recipient", "recipient")
	zendesk.SetIfPresent(ticket, inputs, "external_id", "external_id")
	zendesk.SetIfPresent(ticket, inputs, "assignee_email", "assignee_email")
	zendesk.SetNumericIDIfPresent(ticket, inputs, "group_id", "group_id")
	zendesk.SetNumericIDIfPresent(ticket, inputs, "assignee_id", "assignee_id")
	zendesk.SetCSVIfPresent(ticket, inputs, "tags", "tags")
	if err := zendesk.SetJSONIfPresent(ticket, inputs, "custom_fields", "custom_fields"); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	// A ticket update carries at most one comment. A public reply takes
	// precedence over an internal note when both are supplied.
	if reply := zendesk.OptionalString("public_reply", inputs); reply != "" {
		ticket["comment"] = map[string]interface{}{"body": reply, "public": true}
	} else if note := zendesk.OptionalString("internal_note", inputs); note != "" {
		ticket["comment"] = map[string]interface{}{"html_body": note, "public": false}
	}

	if err := zendesk.MergeAdditionalFields(ticket, inputs); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	resp, err := zendesk.UpdateResource(subdomain, auth, "/tickets/"+url.PathEscape(id)+".json", "ticket", ticket)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ResourceResult(resp, "ticket", fmt.Sprintf("Updated ticket %s", id))
	return out, nil
}
