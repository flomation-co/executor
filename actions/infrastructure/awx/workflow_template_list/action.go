// Package infrastructure_awx_workflow_template_list lists the workflow job
// templates on an AWX / AAP controller.
package infrastructure_awx_workflow_template_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Workflow Templates"
	Description  = "List the workflow templates on your AWX / AAP controller."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+list"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "awx_url", Type: core.ConnectionTypeString, Label: "AWX / AAP URL", Placeholder: "https://awx.example.com — your AWX or Ansible Automation Platform address", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{
		{Name: "API Token (recommended)", Value: "token"},
		{Name: "Username & Password", Value: "basic"},
	}},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "AWX ▸ your user ▸ Tokens ▸ Add, Application blank, Scope = Write. Shown once — copy it then.", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token"}}},
	{Name: "awx_username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your AWX username", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"basic"}}},
	{Name: "awx_password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "your AWX password — note some AWX installs disable password authentication", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"basic"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip certificate verification — only for a self-hosted AWX with a self-signed certificate"},
	{Name: "api_prefix", Type: core.ConnectionTypeString, Label: "API Path Prefix (advanced)", Placeholder: "Leave blank — detected automatically. Only set this if support asks (e.g. /api/controller/v2/)."},

	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Free-text search across name and description"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Exact workflow template name"},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization", Placeholder: "Only workflows in this organization"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Placeholder: "Name (A-Z) by default", Options: []core.ConnectionOption{
		{Name: "Name (A-Z)", Value: "name"},
		{Name: "Name (Z-A)", Value: "-name"},
		{Name: "Newest", Value: "-created"},
		{Name: "Oldest", Value: "created"},
		{Name: "Last run", Value: "-last_job_run"},
	}},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1-based page number — default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "Workflows per page — default 50, max 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow every page until all workflow templates are fetched"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Workflow Templates"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_count", Type: core.ConnectionTypeInteger, Label: "Total Matching"},
	{Name: "has_more", Type: core.ConnectionTypeBoolean, Label: "More Available"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	ctx, cancel := awx.Context()
	defer cancel()

	q, returnAll := awx.ListParams(inputs, "name")
	awx.AddFilter(q, inputs, "search", "search")
	awx.AddFilter(q, inputs, "name", "name")
	awx.AddFilter(q, inputs, "organization", "organization_id")

	items, total, hasMore, err := awx.List(ctx, auth, "workflow_job_templates/", q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Found %d workflow template(s) of %d matching", len(items), total)
	if hasMore {
		summary += " — more remain; tick Return All or ask for the next page"
	}
	return awx.ListResult(items, total, hasMore, summary), nil
}
