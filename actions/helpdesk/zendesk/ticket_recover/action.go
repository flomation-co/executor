package helpdesk_zendesk_ticket_recover

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Recover Suspended Ticket"
	Description  = "Recover a ticket from the suspended queue, turning it into a regular ticket."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+rotate-right"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{Name: "ticket_id", Type: core.ConnectionTypeString, Label: "Suspended Ticket ID", Placeholder: "35436", Required: true},
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

	resp, err := zendesk.ExecuteAPI(subdomain, auth, http.MethodPut, "/suspended_tickets/"+url.PathEscape(id)+"/recover.json", nil)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	if err := zendesk.CheckResponse(resp); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	raw, err := zendesk.DecodeBody(resp)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ResourceResult(raw, "ticket", fmt.Sprintf("Recovered suspended ticket %s", id))
	return out, nil
}
