package helpdesk_intercom_ticket_update

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
	Name         = "Intercom: Update Ticket"
	Description  = "Change an Intercom ticket — edit its title, description, or attributes, move it to another state, snooze or reopen it, or hand it to a different teammate or team."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+pen"
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
	{Name: "ticket_id", Type: core.ConnectionTypeString, Label: "Ticket ID", Placeholder: "The ticket to change — from a Create Ticket step or a search", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "A new short summary for the ticket"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "A new description of what the ticket is about"},
	{Name: "ticket_attributes", Type: core.ConnectionTypeObject, Label: "Ticket Attributes (JSON)", Placeholder: `{"priority":"High"} — attributes defined on the ticket type in Intercom`},
	{Name: "ticket_state_id", Type: core.ConnectionTypeString, Label: "Ticket State", Placeholder: "Move the ticket to this state, e.g. In Progress or Resolved"},
	{Name: "open", Type: core.ConnectionTypeBoolean, Label: "Open", Placeholder: "Tick to reopen the ticket; untick to close it"},
	{Name: "snoozed_until", Type: core.ConnectionTypeDateTime, Label: "Snooze Until", Placeholder: "Hide the ticket until this time, e.g. 2026-07-10T09:00:00Z"},
	{Name: "admin_id", Type: core.ConnectionTypeString, Label: "Acting Admin", Placeholder: "The teammate making this change — required when setting an assignee"},
	{Name: "assignee_id", Type: core.ConnectionTypeString, Label: "Assignee ID", Placeholder: "A teammate or team ID to hand the ticket to — 0 to unassign (needs Acting Admin too)"},
	{Name: "is_shared", Type: core.ConnectionTypeBoolean, Label: "Visible to Customer", Placeholder: "Tick to show the ticket to the contact in the Messenger"},
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
	ticketID, err := intercom.RequiredString("ticket_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}

	// Title and description live inside ticket_attributes under Intercom's
	// built-in _default_title_/_default_description_ keys; an explicit
	// Ticket Attributes object is merged OVER those defaults.
	attrs := map[string]interface{}{}
	if v := intercom.OptionalString("title", inputs); v != "" {
		attrs["_default_title_"] = v
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

	intercom.SetIfPresent(body, inputs, "ticket_state_id", "ticket_state_id")
	intercom.SetBoolIfSet(body, inputs, "open", "open")
	if err := intercom.SetUnixIfPresent(body, inputs, "snoozed_until", "snoozed_until"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	// Live-verified: PUT /tickets rejects string ids here with 400
	// parameter_invalid "Invalid admin_id provided. Id must be an integer."
	// — unlike the conversation-parts endpoints, which accept strings.
	intercom.SetNumericIDIfPresent(body, inputs, "admin_id", "admin_id")
	intercom.SetNumericIDIfPresent(body, inputs, "assignee_id", "assignee_id")
	intercom.SetBoolIfSet(body, inputs, "is_shared", "is_shared")

	if len(body) == 0 {
		return intercom.ErrorResult("provide at least one field to update on the ticket"), nil
	}

	obj, err := intercom.WriteObject(auth, http.MethodPut, "/tickets/"+url.PathEscape(ticketID), body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Updated ticket %s", ticketID)), nil
}
