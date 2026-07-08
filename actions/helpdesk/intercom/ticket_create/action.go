package helpdesk_intercom_ticket_create

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Create Ticket"
	Description  = "Create a new ticket in Intercom. Pick the ticket type, say who it's for (by Contact ID, email, or your external ID), and optionally add a title, description, and an assignee."
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
	{Name: "ticket_type_id", Type: core.ConnectionTypeString, Label: "Ticket Type", Placeholder: "The kind of ticket to create (from your Intercom ticket types)", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Intercom ID of the person this ticket is for"},
	{Name: "contact_email", Type: core.ConnectionTypeString, Label: "Contact Email", Placeholder: "jane@acme.com — used when no Contact ID is given"},
	{Name: "contact_external_id", Type: core.ConnectionTypeString, Label: "Contact External ID", Placeholder: "Your own ID for the person, e.g. their ID in your database"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "A short summary shown on the ticket"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "What the ticket is about"},
	{Name: "ticket_attributes", Type: core.ConnectionTypeObject, Label: "Ticket Attributes (JSON)", Placeholder: `{"priority":"High"} — attributes defined on the ticket type in Intercom`},
	{Name: "company_id", Type: core.ConnectionTypeString, Label: "Company ID", Placeholder: "The Intercom company to file the ticket under"},
	{Name: "admin_assignee_id", Type: core.ConnectionTypeString, Label: "Assign to Teammate", Placeholder: "The teammate to assign the ticket to"},
	{Name: "team_assignee_id", Type: core.ConnectionTypeString, Label: "Assign to Team", Placeholder: "The team to assign the ticket to"},
	{Name: "conversation_to_link_id", Type: core.ConnectionTypeString, Label: "Link to Conversation", Placeholder: "A conversation ID to attach this ticket to"},
	{Name: "created_at", Type: core.ConnectionTypeDateTime, Label: "Created At", Placeholder: "Backdate the ticket, e.g. 2026-07-08T09:00:00Z"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ticket ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Ticket"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	ticketTypeID, err := intercom.RequiredString("ticket_type_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	// The contacts array takes exactly ONE entry identifying who the ticket is
	// for — an Intercom ID, an email, or your external ID, in that priority.
	contactID := intercom.OptionalString("contact_id", inputs)
	contactEmail := intercom.OptionalString("contact_email", inputs)
	contactExternalID := intercom.OptionalString("contact_external_id", inputs)
	var contact map[string]interface{}
	switch {
	case contactID != "":
		contact = map[string]interface{}{"id": contactID}
	case contactEmail != "":
		contact = map[string]interface{}{"email": contactEmail}
	case contactExternalID != "":
		contact = map[string]interface{}{"external_id": contactExternalID}
	default:
		return intercom.ErrorResult("provide a Contact ID, Contact Email, or Contact External ID so Intercom knows who the ticket is for"), nil
	}

	body := map[string]interface{}{
		"ticket_type_id": ticketTypeID,
		"contacts":       []interface{}{contact},
	}

	// Title and description live inside ticket_attributes under Intercom's
	// built-in _default_title_/_default_description_ keys; an explicit
	// Ticket Attributes object is merged OVER those defaults.
	attrs := map[string]interface{}{}
	title := intercom.OptionalString("title", inputs)
	if title != "" {
		attrs["_default_title_"] = title
	}
	if v := intercom.OptionalString("description", inputs); v != "" {
		attrs["_default_description_"] = v
	}
	raw, err := intercom.OptionalJSON("ticket_attributes", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	if raw != nil {
		obj, ok := raw.(map[string]interface{})
		if !ok {
			return intercom.ErrorResult(`ticket_attributes must be a JSON object, e.g. {"priority":"High"}`), nil
		}
		for k, v := range obj {
			attrs[k] = v
		}
	}
	if len(attrs) > 0 {
		body["ticket_attributes"] = attrs
	}

	intercom.SetIfPresent(body, inputs, "company_id", "company_id")
	assignment := map[string]interface{}{}
	intercom.SetIfPresent(assignment, inputs, "admin_assignee_id", "admin_assignee_id")
	intercom.SetIfPresent(assignment, inputs, "team_assignee_id", "team_assignee_id")
	if len(assignment) > 0 {
		body["assignment"] = assignment
	}
	intercom.SetIfPresent(body, inputs, "conversation_to_link_id", "conversation_to_link_id")
	if err := intercom.SetUnixIfPresent(body, inputs, "created_at", "created_at"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/tickets", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	summary := "Created ticket"
	if title != "" {
		summary = fmt.Sprintf("Created ticket %q", title)
	}
	return intercom.ResourceResult(obj, summary), nil
}
