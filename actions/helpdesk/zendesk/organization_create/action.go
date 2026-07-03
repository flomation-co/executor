package helpdesk_zendesk_organization_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Create Organization"
	Description  = "Create an organization in Zendesk. Set details, notes, domains and tags directly, add custom organization fields as JSON, or use Additional Fields."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The organization's name", Required: true},
	{Name: "details", Type: core.ConnectionTypeString, Label: "Details", Placeholder: "Details such as the address"},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes"},
	{Name: "domain_names", Type: core.ConnectionTypeString, Label: "Domain Names", Placeholder: "acme.com, acme.io (comma-separated)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "vip, wholesale (comma-separated)"},
	{Name: "organization_fields", Type: core.ConnectionTypeObject, Label: "Organization Fields (JSON)", Placeholder: `{"tier":"gold"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Organization ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Organization"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	subdomain, auth, err := zendesk.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	name, err := zendesk.RequiredString("name", inputs)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"name": name}
	zendesk.SetIfPresent(body, inputs, "details", "details")
	zendesk.SetIfPresent(body, inputs, "notes", "notes")
	zendesk.SetCSVIfPresent(body, inputs, "domain_names", "domain_names")
	zendesk.SetCSVIfPresent(body, inputs, "tags", "tags")
	if err := zendesk.SetJSONIfPresent(body, inputs, "organization_fields", "organization_fields"); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	if err := zendesk.MergeAdditionalFields(body, inputs); err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	resp, err := zendesk.CreateResource(subdomain, auth, "/organizations.json", "organization", body)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ResourceResult(resp, "organization", "")
	out["tool_result"] = fmt.Sprintf("Created organization %s", out["id"])
	return out, nil
}
