package helpdesk_zendesk_user_get_organizations

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Get User's Organizations"
	Description  = "List the organizations a Zendesk user belongs to."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+user-group"
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
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Organizations"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "next_page", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
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

	path := "/users/" + url.PathEscape(id) + "/organizations.json"
	items, next, lastRaw, _, err := zendesk.ListResources(subdomain, auth, path, "organizations", url.Values{}, true)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ListResult(items, next, lastRaw, fmt.Sprintf("Found %d organization(s) for user %s", len(items), id))
	return out, nil
}
