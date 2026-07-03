package helpdesk_zendesk_ticket_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Get Ticket"
	Description  = "Retrieve a single Zendesk ticket by its ID. Choose Regular for a normal ticket or Suspended for one held in the suspended queue."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+eye"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{
		Name:  "ticket_type",
		Type:  core.ConnectionTypeString,
		Label: "Ticket Type",
		Options: []core.ConnectionOption{
			{Name: "Regular", Value: "regular"},
			{Name: "Suspended", Value: "suspended"},
		},
	},
	{Name: "ticket_id", Type: core.ConnectionTypeString, Label: "Ticket ID", Placeholder: "35436", Required: true},
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

	// Default to a regular ticket when the type is left unset.
	suspended := zendesk.OptionalString("ticket_type", inputs) == "suspended"
	path := "/tickets/" + url.PathEscape(id) + ".json"
	key := "ticket"
	if suspended {
		path = "/suspended_tickets/" + url.PathEscape(id) + ".json"
		key = "suspended_ticket"
	}

	resp, err := zendesk.GetResource(subdomain, auth, path, url.Values{})
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ResourceResult(resp, key, fmt.Sprintf("Retrieved ticket %s", id))
	return out, nil
}
