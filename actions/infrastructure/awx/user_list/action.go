// Package infrastructure_awx_user_list lists the users on an AWX / AAP
// controller.
//
// Two AWX details shape this action:
//
//   - Scoping by organization is a DIFFERENT ENDPOINT, not a filter: users are
//     linked to an organization through a role, not a foreign key, so ?organization=
//     on /users/ does nothing. The sublist /organizations/{id}/users/ is what
//     answers "who is in this org".
//   - A user's `password` always serialises as the literal "$encrypted$" — never a
//     hash and never the password — so a user record is safe to emit whole.
package infrastructure_awx_user_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	awx "flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Users"
	Description  = "List the users on your AWX / AAP controller."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+list"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// ---- AUTH BLOCK (identical on every AWX action; see awx.AuthInputs) ----
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

	// ---- FILTERS ----
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization", Placeholder: "Blank lists every user you can see"},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Free-text search across username, first name, last name and email"},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "Exact username"},
	{Name: "is_superuser", Type: core.ConnectionTypeString, Label: "Superusers", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Superusers only", Value: "true"},
	}},
	// NOTE: no "created" ordering here, unlike the other AWX collections. A user's
	// `created` is a serializer method over Django's date_joined, not a column on
	// the model, so AWX answers order_by=created with a 400.
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Options: []core.ConnectionOption{
		{Name: "Username", Value: "username"},
		{Name: "Username (Z-A)", Value: "-username"},
		{Name: "Email", Value: "email"},
		{Name: "Newest", Value: "-id"},
		{Name: "Oldest", Value: "id"},
	}, Placeholder: "Default: Username"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Which page to fetch — default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "How many per page — default 50, maximum 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Fetch every page instead of just one"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Users"},
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
		return nil, err // the ONLY hard failure: the node is mis-configured
	}

	ctx, cancel := awx.Context()
	defer cancel()

	q, returnAll := awx.ListParams(inputs, "username")
	awx.AddFilter(q, inputs, "search", "search")
	awx.AddFilter(q, inputs, "username", "username")
	awx.AddFilter(q, inputs, "is_superuser", "is_superuser")

	// Org membership is a ROLE, not a column — the sublist is the only way to ask
	// "who is in this organization".
	path := "users/"
	scope := ""
	if orgID := awx.OptionalString("organization_id", inputs); orgID != "" {
		id, err := awx.RequiredInt("organization_id", "Organization", inputs)
		if err != nil {
			return awx.ErrorResult(err.Error()), nil
		}
		path = fmt.Sprintf("organizations/%d/users/", id)
		scope = fmt.Sprintf(" in organization %d", id)
	}

	items, total, hasMore, err := awx.List(ctx, auth, path, q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Found %d user(s)%s", len(items), scope)
	if hasMore {
		summary += fmt.Sprintf(" of %d — more remain; tick Return All or ask for the next page", total)
	}
	return awx.ListResult(items, total, hasMore, summary), nil
}
