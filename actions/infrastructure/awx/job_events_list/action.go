// Package infrastructure_awx_job_events_list lists the per-task events of an AWX
// job or ad-hoc command.
package infrastructure_awx_job_events_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Job Events"
	Description  = "List the per-task events of an AWX job — which host ran which task, and what it returned. Filter to just the failures to find what broke."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+list"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

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
	// A workflow job has no events of its own — its steps are workflow nodes, and
	// the events live on the child jobs.
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Type", Placeholder: "Which kind of AWX job — a playbook Job unless you know otherwise", Options: []core.ConnectionOption{
		{Name: "Job", Value: "job"},
		{Name: "Ad-Hoc Command", Value: "ad_hoc_command"},
	}},
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Placeholder: "42 — the AWX job whose events you want", Required: true},
	{Name: "failed_only", Type: core.ConnectionTypeBoolean, Label: "Failed Only", Placeholder: "Only the events that failed — the fastest way to find what broke"},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event Type", Placeholder: "e.g. runner_on_failed, playbook_on_stats — blank for all"},
	{Name: "host_name", Type: core.ConnectionTypeString, Label: "Host", Placeholder: "Only events for this host — blank for all"},
	{Name: "no_truncate", Type: core.ConnectionTypeBoolean, Label: "Full Output", Placeholder: "Return each event's full output instead of the first 1024 bytes"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Which page of results — default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "Events per page — default 50, maximum 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow every page until all matching events are fetched — a big playbook has thousands"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Events"},
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
	if kind == "" {
		kind = awx.JobKindJob
	}

	id, err := awx.RequiredInt("job_id", "Job ID", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// Ad-hoc events live at /ad_hoc_commands/{id}/events/ — NOT at a top-level
	// /ad_hoc_command_events/ collection — and their serializer is narrower (no
	// playbook, play, task or role).
	var path string
	switch kind {
	case awx.JobKindJob:
		path = fmt.Sprintf("jobs/%d/job_events/", id)
	case awx.JobKindAdHocCommand:
		path = fmt.Sprintf("ad_hoc_commands/%d/events/", id)
	default:
		return awx.ErrorResult(fmt.Sprintf("Job Type %q has no per-task events — choose Job or Ad-Hoc Command. (A workflow's steps are workflow nodes: use List Workflow Job Nodes, then this action on the child job.)", kind)), nil
	}

	// ★ ListParams maps Page Size to page_size and NEVER to limit. Sending ?limit=
	// to an events endpoint silently switches AWX from UnifiedJobEventPagination to
	// LimitPagination, and the response loses count / next / previous entirely — so
	// pagination and the total both break, with no error to say why.
	q, returnAll := awx.ListParams(inputs, "")

	if awx.BoolInput("failed_only", inputs) {
		q.Set("failed", "true")
	}
	awx.AddFilter(q, inputs, "event", "event")
	awx.AddFilter(q, inputs, "host_name", "host_name")
	if awx.BoolInput("no_truncate", inputs) {
		// Each event's stdout is otherwise clipped to 1024 bytes
		// (EVENT_STDOUT_MAX_BYTES_DISPLAY) with a trailing ellipsis.
		q.Set("no_truncate", "true")
	}

	items, total, hasMore, err := awx.List(ctx, auth, path, q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	scope := "event"
	if awx.BoolInput("failed_only", inputs) {
		scope = "failed event"
	}
	summary := fmt.Sprintf("Found %d %s(s) of %d matching on %s %d.", len(items), scope, total, label(kind), id)
	if hasMore {
		summary += " More remain — tick Return All, or ask for the next page."
	}
	if !awx.BoolInput("no_truncate", inputs) {
		summary += " Each event's output is clipped to 1024 bytes; tick Full Output for all of it."
	}

	return awx.ListResult(items, total, hasMore, summary), nil
}

func label(kind string) string {
	if kind == awx.JobKindAdHocCommand {
		return "ad-hoc command"
	}
	return "job"
}
