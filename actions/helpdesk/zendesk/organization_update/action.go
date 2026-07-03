package helpdesk_zendesk_organization_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	zendesk "flomation.app/automate/executor/actions/helpdesk/zendesk"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Zendesk: Update Organization"
	Description  = "Update a Zendesk organization. Change details, notes, domains, tags, custom organization fields, or any other field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "zendesk+pencil"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "subdomain", Type: core.ConnectionTypeString, Label: "Subdomain", Placeholder: "mycompany (from mycompany.zendesk.com)", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Agent Email", Placeholder: "you@company.com (paired with the API token)"},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Zendesk API token"},
	{Name: "oauth_token", Type: core.ConnectionTypeSecret, Label: "OAuth Access Token", Placeholder: "Optional — a bearer token used instead of the email + API token"},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization ID", Placeholder: "35436", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The organization's display name"},
	{Name: "details", Type: core.ConnectionTypeString, Label: "Details", Placeholder: "Additional details, e.g. account number"},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes", Placeholder: "Freeform notes about the organization"},
	{Name: "domain_names", Type: core.ConnectionTypeString, Label: "Domain Names", Placeholder: "example.com, example.org (comma-separated)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "vip, wholesale (comma-separated)"},
	{Name: "organization_fields", Type: core.ConnectionTypeObject, Label: "Organization Fields (JSON)", Placeholder: `{"support_plan":"gold"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"group_id":123}`},
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
	id, err := zendesk.RequiredString("organization_id", inputs)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	zendesk.SetIfPresent(body, inputs, "name", "name")
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

	resp, err := zendesk.UpdateResource(subdomain, auth, "/organizations/"+url.PathEscape(id)+".json", "organization", body)
	if err != nil {
		return zendesk.ErrorResult(err.Error()), nil
	}
	out := zendesk.ResourceResult(resp, "organization", fmt.Sprintf("Updated organization %s", id))
	return out, nil
}
