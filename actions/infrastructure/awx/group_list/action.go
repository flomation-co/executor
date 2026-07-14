// Package infrastructure_awx_group_list lists the groups of an AWX inventory.
//
// Groups are how an inventory is carved up ("webservers", "db", "prod") and they
// are what a playbook's `hosts:` line names, so listing them is usually the first
// thing a flow does before it targets a run at one.
//
// Inventory is a FILTER here, not a path segment: AWX serves every group of every
// inventory from one collection at {root}groups/, narrowed with ?inventory=<id>.
// Leaving Inventory blank therefore lists the groups of the WHOLE controller,
// which is a legitimate thing to want and is why the input is not required.
package infrastructure_awx_group_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Groups"
	Description  = "List the groups in an inventory."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+list"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// ---- AUTH BLOCK (awx.AuthInputs, verbatim — see awx_inputs_drift_test.go) ----
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
	// ---- END AUTH BLOCK ----

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Pick the inventory whose groups you want — leave blank to list every group on the controller"},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Free-text search across name and description"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Exact group name, e.g. webservers"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Placeholder: "Default: Name (A-Z)", Options: []core.ConnectionOption{
		{Name: "Name (A-Z)", Value: "name"},
		{Name: "Name (Z-A)", Value: "-name"},
		{Name: "Newest first", Value: "-created"},
		{Name: "Oldest first", Value: "created"},
	}},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Which page of results to fetch — default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "Groups per page — default 50, maximum 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow every page until all groups are fetched (up to 10,000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Groups"},
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

	// Groups sort by name by default: a human scanning "db, prod, webservers" is
	// the point, whereas AWX's own id-ascending default is creation order.
	q, returnAll := awx.ListParams(inputs, "name")
	awx.AddFilter(q, inputs, "inventory", "inventory_id")
	awx.AddFilter(q, inputs, "search", "search")
	awx.AddFilter(q, inputs, "name", "name")

	items, total, hasMore, err := awx.List(ctx, auth, "groups/", q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Found %d group(s)", len(items))
	if hasMore {
		summary = fmt.Sprintf("Found %d of %d group(s) — more remain; tick Return All or ask for the next page", len(items), total)
	}
	return awx.ListResult(items, total, hasMore, summary), nil
}
