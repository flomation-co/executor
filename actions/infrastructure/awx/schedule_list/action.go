// Package infrastructure_awx_schedule_list lists the schedules on an AWX / AAP
// controller.
//
// A schedule is AWX's own recurring launch of a job template. Listing them is
// how an operator answers "what is this controller going to run on its own, and
// when?" — hence the default ordering by next_run rather than by name.
package infrastructure_awx_schedule_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	awx "flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Schedules"
	Description  = "List the schedules on your AWX / AAP controller and when each one next runs."
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
	{Name: "job_template_id", Type: core.ConnectionTypeString, Label: "Job Template", Placeholder: "Only the schedules of this job template — blank lists every schedule"},
	{Name: "enabled", Type: core.ConnectionTypeString, Label: "Enabled", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Enabled", Value: "true"},
		{Name: "Disabled", Value: "false"},
	}},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Free-text search across name and description"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Options: []core.ConnectionOption{
		{Name: "Next run", Value: "next_run"},
		{Name: "Name", Value: "name"},
		{Name: "Name (Z-A)", Value: "-name"},
	}, Placeholder: "Default: Next run"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Which page to fetch — default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "How many per page — default 50, maximum 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Fetch every page instead of just one"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Schedules"},
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

	// order_by defaults to next_run: "when will this fire" is the question a
	// schedule list is being asked.
	q, returnAll := awx.ListParams(inputs, "next_run")

	// A schedule's foreign key is unified_job_template — it can point at a job
	// template, a workflow, a project or an inventory source. The picker on this
	// node offers job templates, which is the case that matters.
	if id := awx.OptionalString("job_template_id", inputs); id != "" {
		q.Set("unified_job_template", id)
	}
	awx.AddFilter(q, inputs, "enabled", "enabled")
	awx.AddFilter(q, inputs, "search", "search")

	items, total, hasMore, err := awx.List(ctx, auth, "schedules/", q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Found %d schedule(s)", len(items))
	if hasMore {
		summary += fmt.Sprintf(" of %d — more remain; tick Return All or ask for the next page", total)
	}
	return awx.ListResult(items, total, hasMore, summary), nil
}
