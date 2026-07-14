// Package infrastructure_awx_inventory_source_list lists the external sources an
// inventory imports its hosts from.
package infrastructure_awx_inventory_source_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Inventory Sources"
	Description  = "List the external sources an inventory imports its hosts from."
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Leave blank to list the sources of every inventory"},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Free-text search across name and description"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Options: []core.ConnectionOption{
		{Name: "Name", Value: "name"},
		{Name: "Name (Z-A)", Value: "-name"},
		{Name: "Newest", Value: "-created"},
		{Name: "Oldest", Value: "created"},
		{Name: "Last sync", Value: "-last_job_run"},
	}, Placeholder: "Default: Name"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "How many to return per page — default 50, maximum 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Fetch every page until they are all in — ignores Page"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Inventory Sources"},
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
	awx.AddFilter(q, inputs, "inventory", "inventory_id")
	awx.AddFilter(q, inputs, "search", "search")

	items, total, hasMore, err := awx.List(ctx, auth, "inventory_sources/", q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	scope := "across every inventory"
	if id := awx.OptionalString("inventory_id", inputs); id != "" {
		scope = "in inventory " + id
	}
	summary := fmt.Sprintf("Found %d inventory source(s) %s (%d matching in AWX)", len(items), scope, total)
	if hasMore {
		summary += " — more remain; tick Return All or ask for the next page"
	}

	return awx.ListResult(items, total, hasMore, summary), nil
}
