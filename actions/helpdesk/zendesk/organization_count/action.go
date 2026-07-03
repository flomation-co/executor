package helpdesk_zendesk_organization_count

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Count Organizations"
	Description  = "Return the number of organizations in your Zendesk account."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+hashtag"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
}

var Outputs = [...]core.Connection{
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	raw, err := zendesk.GetResource(subdomain, auth, "/organizations/count.json", url.Values{})
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	// Zendesk wraps the total in {"count": {"value": N, "refreshed_at": ...}}.
	var value int
	if countObj, ok := raw["count"].(map[string]interface{}); ok {
		if v, ok := countObj["value"].(float64); ok {
			value = int(v)
		}
	}
	return map[string]interface{}{
		"count":       value,
		"result":      raw,
		"tool_result": fmt.Sprintf("Counted %d organization(s)", value),
		"success":     true,
		"error":       "",
	}, nil
}
