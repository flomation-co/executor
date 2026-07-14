// Package infrastructure_awx_inventory_list lists the inventories on an AWX /
// AAP controller.
//
// The Kind filter deserves a word. AWX stores a standard inventory's kind as the
// EMPTY STRING (only smart and constructed inventories carry a value), so "no
// filter" and "only standard inventories" would both be the empty string on the
// wire — indistinguishable. The dropdown therefore carries the sentinel value
// "standard", which this action translates into an explicit `?kind=` (present,
// empty) before it goes to AWX. Any other option is passed through as-is, and an
// untouched dropdown sends no kind filter at all.
package infrastructure_awx_inventory_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Inventories"
	Description  = "List the inventories on your AWX / AAP controller."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+list"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

// kindStandard is the sentinel the Kind dropdown uses for "standard inventories
// only", because AWX's own value for that is the empty string — which the editor
// cannot tell apart from an untouched dropdown.
const kindStandard = "standard"

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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Exact inventory name — use Search for a partial match"},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization", Placeholder: "Only inventories in this organization — blank returns all of them"},
	{Name: "kind", Type: core.ConnectionTypeString, Label: "Kind", Placeholder: "Blank returns every kind of inventory", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Standard", Value: kindStandard},
		{Name: "Smart", Value: "smart"},
		{Name: "Constructed", Value: "constructed"},
	}},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Placeholder: "Name (A–Z) by default", Options: []core.ConnectionOption{
		{Name: "Name (A–Z)", Value: "name"},
		{Name: "Name (Z–A)", Value: "-name"},
		{Name: "Newest first", Value: "-created"},
		{Name: "Oldest first", Value: "created"},
		{Name: "Recently changed", Value: "-modified"},
	}},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Which page of results to fetch — default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "How many per page — default 50, maximum 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Fetch every page instead of just one — slower on a large AWX"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Inventories"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned"},
	{Name: "total_count", Type: core.ConnectionTypeInteger, Label: "Total Matching"},
	{Name: "has_more", Type: core.ConnectionTypeBoolean, Label: "More Pages Available"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	q, returnAll := awx.ListParams(inputs, "name")
	awx.AddFilter(q, inputs, "search", "search")
	awx.AddFilter(q, inputs, "name", "name")
	awx.AddFilter(q, inputs, "organization", "organization_id")

	// AddFilter would drop the empty string, which is exactly the value a
	// standard inventory has — so the standard case is set explicitly.
	switch kind := awx.OptionalString("kind", inputs); kind {
	case "":
		// No filter: every kind.
	case kindStandard:
		q.Set("kind", "")
	default:
		q.Set("kind", kind)
	}

	ctx, cancel := awx.Context()
	defer cancel()

	items, total, hasMore, err := awx.List(ctx, auth, "inventories/", q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.ListResult(items, total, hasMore,
		fmt.Sprintf("Found %d of %d inventories", len(items), total)), nil
}
