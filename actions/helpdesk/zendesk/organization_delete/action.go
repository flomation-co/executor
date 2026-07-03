package helpdesk_zendesk_organization_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Delete Organization"
	Description  = "Permanently delete a Zendesk organization by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+trash"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization ID", Placeholder: "35436", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Organization ID"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	subdomain, auth, err := zendesk.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := zendesk.RequiredString("organization_id", inputs)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	path := "/organizations/" + url.PathEscape(id) + ".json"
	if err := zendesk.DeleteResource(subdomain, auth, path); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"id":          id,
		"tool_result": fmt.Sprintf("Deleted organization %s", id),
		"success":     true,
		"error":       "",
	}, nil
}
