package helpdesk_zendesk_user_get_related_data

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Get User Related Data"
	Description  = "Get counts of data related to a Zendesk user (assigned tickets, requested tickets, organization subscriptions, etc.)."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+list"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID", Placeholder: "35436", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User Related Data"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	subdomain, auth, err := zendesk.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := zendesk.RequiredString("user_id", inputs)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	path := "/users/" + url.PathEscape(id) + "/related.json"
	resp, err := zendesk.GetResource(subdomain, auth, path, url.Values{})
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ResourceResult(resp, "user_related", fmt.Sprintf("Retrieved related data for user %s", id))
	return out, nil
}
