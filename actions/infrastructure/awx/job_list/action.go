// Package infrastructure_awx_job_list lists AWX jobs, newest first.
package infrastructure_awx_job_list

import (
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Jobs"
	Description  = "List AWX jobs — filter by job template, status or time, newest first."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+list"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

// defaultOrderBy is the ordering the operator gets when they leave Order By blank.
//
// ★ AWX's OWN default ordering on /jobs/, /workflow_jobs/ and /ad_hoc_commands/ is
// id ASCENDING (UnifiedJob's Meta.ordering; the views set none), so an unordered
// list hands back the OLDEST jobs first — never what a human means by "list my
// jobs". Newest-first is forced here.
const defaultOrderBy = "-created"

var Inputs = [...]core.Connection{
	// ---- AUTH (re-declared verbatim from awx.AuthInputs) ----
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

	// ---- ACTION ----
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Type", Placeholder: "Which kind of AWX job to list — playbook Jobs unless you know otherwise", Options: []core.ConnectionOption{
		{Name: "Job", Value: "job"},
		{Name: "Workflow Job", Value: "workflow_job"},
		{Name: "Ad-Hoc Command", Value: "ad_hoc_command"},
	}},
	{Name: "job_template_id", Type: core.ConnectionTypeString, Label: "Job Template", Placeholder: "Only jobs launched from this template — leave blank for all", Visible: &core.VisibleWhen{Field: "job_kind", Values: []string{"", "job"}}},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Only jobs in this state — leave blank for any", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Successful", Value: "successful"},
		{Name: "Failed", Value: "failed"},
		{Name: "Errored", Value: "error"},
		{Name: "Canceled", Value: "canceled"},
		{Name: "Running", Value: "running"},
		{Name: "Pending", Value: "pending"},
	}},
	{Name: "launch_type", Type: core.ConnectionTypeString, Label: "Launch Type", Placeholder: "How the job was started — leave blank for any", Options: []core.ConnectionOption{
		{Name: "Manual", Value: "manual"},
		{Name: "Relaunch", Value: "relaunch"},
		{Name: "Scheduled", Value: "scheduled"},
		{Name: "Webhook", Value: "webhook"},
		{Name: "Workflow", Value: "workflow"},
		{Name: "Callback", Value: "callback"},
		{Name: "Dependency", Value: "dependency"},
	}},
	{Name: "finished_after", Type: core.ConnectionTypeDateTime, Label: "Finished After", Placeholder: "Only jobs that finished after this moment"},
	{Name: "created_after", Type: core.ConnectionTypeDateTime, Label: "Created After", Placeholder: "Only jobs created after this moment"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name Contains", Placeholder: "Deploy — matches any job whose name contains this"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By", Placeholder: "Newest first unless you change it", Options: []core.ConnectionOption{
		{Name: "Newest first", Value: "-created"},
		{Name: "Oldest first", Value: "created"},
		{Name: "Newest finished", Value: "-finished"},
	}},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Which page of results — default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "Jobs per page — default 50, maximum 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow every page until all matching jobs are fetched"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Jobs"},
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

	kind := awx.OptionalString("job_kind", inputs)
	switch kind {
	case "", awx.JobKindJob, awx.JobKindWorkflowJob, awx.JobKindAdHocCommand:
	default:
		return awx.ErrorResult(fmt.Sprintf("Job Type %q cannot be listed here — choose Job, Workflow Job or Ad-Hoc Command.", kind)), nil
	}
	path, err := awx.JobKindPath(kind)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if kind == "" {
		kind = awx.JobKindJob
	}

	q, returnAll := awx.ListParams(inputs, defaultOrderBy)

	// A job template only exists on the /jobs/ collection — a workflow job is
	// launched from a workflow template and an ad-hoc command from none at all,
	// which is why the input is hidden for those kinds.
	if kind == awx.JobKindJob {
		if id, err := templateID(inputs); err != nil {
			return awx.ErrorResult(err.Error()), nil
		} else if id > 0 {
			q.Set("job_template", strconv.FormatInt(id, 10))
		}
	}

	if status := awx.OptionalString("status", inputs); status != "" {
		q.Set("status__in", status)
	}
	awx.AddFilter(q, inputs, "launch_type", "launch_type")
	awx.AddFilter(q, inputs, "finished__gt", "finished_after")
	awx.AddFilter(q, inputs, "created__gt", "created_after")
	awx.AddFilter(q, inputs, "name__icontains", "name")

	// NB: there is deliberately no filter on the job's own `limit` field. `limit` is
	// a RESERVED query-param name in AWX's filter backend — it is swallowed as a
	// pagination hint — so ?limit= cannot filter jobs by their Ansible limit.

	items, total, hasMore, err := awx.List(ctx, auth, path, q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Found %d %s(s) of %d matching, newest first.", len(items), label(kind), total)
	if order := awx.OptionalString("order_by", inputs); order != "" && order != defaultOrderBy {
		summary = fmt.Sprintf("Found %d %s(s) of %d matching, ordered by %s.", len(items), label(kind), total, order)
	}
	if hasMore {
		summary += " More remain — tick Return All, or ask for the next page."
	}
	summary += " AWX strips the traceback from list responses; use Get Job for the full record of one job."

	return awx.ListResult(items, total, hasMore, summary), nil
}

// templateID reads the Job Template input, which a live dropdown fills with the
// template's AWX id as a string.
func templateID(inputs []*core.Connection) (int64, error) {
	if awx.OptionalString("job_template_id", inputs) == "" {
		return 0, nil
	}
	return awx.RequiredInt("job_template_id", "Job Template", inputs)
}

func label(kind string) string {
	switch kind {
	case awx.JobKindWorkflowJob:
		return "workflow job"
	case awx.JobKindAdHocCommand:
		return "ad-hoc command"
	default:
		return "job"
	}
}
