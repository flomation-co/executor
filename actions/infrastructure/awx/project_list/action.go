// Package infrastructure_awx_project_list lists the projects on an AWX / AAP
// controller.
package infrastructure_awx_project_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Projects"
	Description  = "List the projects on your AWX / AAP controller and their last sync status."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+list"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction

	// scmTypeManual is the DROPDOWN sentinel for a manual project, not the value
	// sent to AWX. AWX's own value for "manual" is the EMPTY STRING — which is
	// also what an untouched dropdown yields, so the two would be
	// indistinguishable. The sentinel keeps "Any" (send no filter) apart from
	// "Manual" (send scm_type=); resolveSCMType translates it.
	scmTypeManual = "manual"
)

var Inputs = [...]core.Connection{
	// --- AWX credential block (identical in all 59 AWX actions) ---
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

	// --- filters ---
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Free-text search across name and description"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Exact project name"},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization", Placeholder: "Only projects in this organization"},
	{Name: "scm_type", Type: core.ConnectionTypeString, Label: "Source Control Type", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Git", Value: "git"},
		{Name: "Subversion", Value: "svn"},
		{Name: "Insights", Value: "insights"},
		{Name: "Remote Archive", Value: "archive"},
		{Name: "Manual (no source control)", Value: scmTypeManual},
	}},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Last Sync Status", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Successful", Value: "successful"},
		{Name: "Failed", Value: "failed"},
		{Name: "Error", Value: "error"},
		{Name: "Canceled", Value: "canceled"},
		{Name: "Running", Value: "running"},
		{Name: "Pending", Value: "pending"},
		{Name: "Never Updated", Value: "never updated"},
	}},

	// --- paging ---
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Options: []core.ConnectionOption{
		{Name: "Name", Value: "name"},
		{Name: "Name (Z-A)", Value: "-name"},
		{Name: "Newest", Value: "-created"},
		{Name: "Oldest", Value: "created"},
		{Name: "Last synced", Value: "-last_job_run"},
	}},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Which page to fetch — default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "Projects per page — default 50, maximum 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow every page until all projects are fetched"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Projects"},
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
		return nil, err // the ONE hard failure: the node is mis-configured
	}

	ctx, cancel := awx.Context()
	defer cancel()

	// AWX's own default ordering is by id, i.e. oldest first, which is never what
	// a human wants from a picker.
	q, returnAll := awx.ListParams(inputs, "name")
	awx.AddFilter(q, inputs, "search", "search")
	awx.AddFilter(q, inputs, "name", "name")
	awx.AddFilter(q, inputs, "organization", "organization_id")
	awx.AddFilter(q, inputs, "status", "status")
	applySCMTypeFilter(q, inputs)

	items, total, hasMore, err := awx.List(ctx, auth, "projects/", q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.ListResult(items, total, hasMore, fmt.Sprintf("Found %d project(s) of %d matching", len(items), total)), nil
}

// applySCMTypeFilter sets ?scm_type=, translating the Manual sentinel.
//
// AWX stores a manual project's scm_type as the EMPTY STRING, so filtering for
// manual projects means sending an EMPTY query value — which awx.AddFilter (and
// any "skip if blank" helper) would drop. Hence the explicit set.
func applySCMTypeFilter(q url.Values, inputs []*core.Connection) {
	switch v := awx.OptionalString("scm_type", inputs); v {
	case "": // "Any" — no filter at all
	case scmTypeManual:
		q.Set("scm_type", "")
	default:
		q.Set("scm_type", v)
	}
}
