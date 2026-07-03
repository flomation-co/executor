package helpdesk_zendesk_ticket_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Create Ticket"
	Description  = "Create a support ticket in Zendesk. The Description becomes the ticket's first comment; set common fields directly or add any other ticket field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+plus"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "The ticket's first comment", Required: true},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject"},
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
	{Name: "requester_id", Type: core.ConnectionTypeString, Label: "Requester ID", Placeholder: "User ID of the ticket requester"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "External ID", Placeholder: "An ID linking this ticket to a record in another system"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "vip, refund (comma-separated)"},
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

	description, err := zendesk.RequiredString("description", inputs)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	ticket := map[string]interface{}{
		"comment": map[string]interface{}{"body": description},
	}
	zendesk.SetIfPresent(ticket, inputs, "subject", "subject")
	zendesk.SetIfPresent(ticket, inputs, "type", "type")
	zendesk.SetIfPresent(ticket, inputs, "priority", "priority")
	zendesk.SetIfPresent(ticket, inputs, "status", "status")
	zendesk.SetIfPresent(ticket, inputs, "recipient", "recipient")
	zendesk.SetIfPresent(ticket, inputs, "external_id", "external_id")
	zendesk.SetNumericIDIfPresent(ticket, inputs, "group_id", "group_id")
	zendesk.SetNumericIDIfPresent(ticket, inputs, "assignee_id", "assignee_id")
	zendesk.SetNumericIDIfPresent(ticket, inputs, "requester_id", "requester_id")
	zendesk.SetCSVIfPresent(ticket, inputs, "tags", "tags")
	if err := zendesk.SetJSONIfPresent(ticket, inputs, "custom_fields", "custom_fields"); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	if err := zendesk.MergeAdditionalFields(ticket, inputs); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	resp, err := zendesk.CreateResource(subdomain, auth, "/tickets.json", "ticket", ticket)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ResourceResult(resp, "ticket", "")
	out["tool_result"] = fmt.Sprintf("Created ticket %s", out["id"])
	return out, nil
}
