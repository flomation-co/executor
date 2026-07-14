// Package infrastructure_awx_host_list lists the hosts AWX knows about,
// optionally narrowed to one inventory and/or one group.
//
// group_id maps to AWX's ?groups__id= filter (the reverse relation from Host to
// Group), not to a group sublist endpoint — so a host can be found by group
// without knowing which inventory it lives in. The Inventory input above it is
// what scopes the Group and Host live dropdowns in the editor.
package infrastructure_awx_host_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Hosts"
	Description  = "List the hosts in an inventory, optionally only those in one group."
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Pick the inventory to list hosts from — leave blank for every inventory you can see"},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "Only hosts in this group — pick the Inventory above first"},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Free-text search across host name and description"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Exact host name, e.g. web01.example.com"},
	{Name: "enabled", Type: core.ConnectionTypeString, Label: "Enabled", Placeholder: "Only enabled or only disabled hosts — default Any", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Enabled", Value: "true"},
		{Name: "Disabled", Value: "false"},
	}},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Placeholder: "AWX field to sort on — default name. Prefix with - to reverse, e.g. -created"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Page number (default 1). Ignored when Return All is ticked."},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "Hosts per page (default 50, max 200)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow every page until all matching hosts are fetched (up to 10,000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Hosts"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned"},
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
	// The reverse relation Host -> Group. NOT /groups/{id}/hosts/, so a group can
	// be filtered on without also naming its inventory.
	awx.AddFilter(q, inputs, "groups__id", "group_id")
	awx.AddFilter(q, inputs, "search", "search")
	awx.AddFilter(q, inputs, "name", "name")
	awx.AddFilter(q, inputs, "enabled", "enabled")

	items, total, hasMore, err := awx.List(ctx, auth, "hosts/", q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Found %d host(s)", len(items))
	if hasMore {
		summary = fmt.Sprintf("Found %d host(s) of %d matching — more remain; tick Return All or ask for the next page", len(items), total)
	}
	return awx.ListResult(items, total, hasMore, summary), nil
}
