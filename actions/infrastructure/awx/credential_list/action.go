// Package infrastructure_awx_credential_list lists the credentials stored on the
// AWX / AAP controller.
//
// No secret ever leaves AWX here — Credential.display_inputs() replaces the value
// of every input the credential type marks secret with the literal string
// "$encrypted$" before serialising. That is AWX's placeholder, not a redaction
// this node applied, and the summary says so: an operator who sees "$encrypted$"
// in the results must not conclude the value is literally that.
package infrastructure_awx_credential_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Credentials"
	Description  = "List the credentials stored on your AWX / AAP controller. Secret values are never returned."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Exact credential name"},
	{Name: "credential_type_id", Type: core.ConnectionTypeString, Label: "Credential Type", Placeholder: "Only credentials of this type — e.g. Machine, Amazon Web Services"},
	{Name: "kind", Type: core.ConnectionTypeString, Label: "Kind", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Machine (SSH)", Value: "ssh"},
		{Name: "Vault", Value: "vault"},
		{Name: "Source Control", Value: "scm"},
		{Name: "Cloud", Value: "cloud"},
		{Name: "Container Registry", Value: "registry"},
		{Name: "Network", Value: "net"},
		{Name: "Kubernetes", Value: "kubernetes"},
	}, Placeholder: "A broad category, rather than one specific credential type"},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization", Placeholder: "Only credentials owned by this organization"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Options: []core.ConnectionOption{
		{Name: "Name", Value: "name"},
		{Name: "Name (Z-A)", Value: "-name"},
		{Name: "Newest", Value: "-created"},
		{Name: "Oldest", Value: "created"},
	}, Placeholder: "Default: Name"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "How many to return per page — default 50, maximum 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Fetch every page until they are all in — ignores Page"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Credentials"},
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
	awx.AddFilter(q, inputs, "credential_type", "credential_type_id")
	// credential_type__kind, not AWX's bare ?kind= — the bare filter maps to the
	// type's NAMESPACE (aws, gce, …), whereas the values this node offers (ssh,
	// vault, scm, cloud, registry, net, kubernetes) are CredentialType.kind. Same
	// filter the api's awx-machine-credentials / awx-scm-credentials pickers use.
	awx.AddFilter(q, inputs, "credential_type__kind", "kind")
	awx.AddFilter(q, inputs, "organization", "organization_id")

	items, total, hasMore, err := awx.List(ctx, auth, "credentials/", q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf(
		"Found %d credential(s) (%d matching in AWX). Secret fields read back as the literal text \"$encrypted$\" — that is AWX hiding the value, not the value itself; AWX never returns a stored secret.",
		len(items), total)
	if hasMore {
		summary += " More remain — tick Return All or ask for the next page."
	}

	return awx.ListResult(items, total, hasMore, summary), nil
}
