// Package infrastructure_awx_job_get fetches one AWX job — of any of the five
// "unified job" kinds — by ID.
package infrastructure_awx_job_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Job"
	Description  = "Fetch one AWX job by ID — its status, timings, host results, artifacts and (if it errored) the traceback."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+eye"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// ---- AUTH (re-declared verbatim from awx.AuthInputs; the manifest AST-parser
	// cannot see through a package-level variable) ----
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
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Type", Placeholder: "Which kind of AWX job — a playbook Job unless you know otherwise", Options: []core.ConnectionOption{
		{Name: "Job", Value: "job"},
		{Name: "Workflow Job", Value: "workflow_job"},
		{Name: "Ad-Hoc Command", Value: "ad_hoc_command"},
		{Name: "Project Sync", Value: "project_update"},
		{Name: "Inventory Sync", Value: "inventory_update"},
	}},
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Placeholder: "42 — the AWX job ID, e.g. from a Launch or Wait node", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Job ID"},
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID"},
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Type"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Job"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "elapsed", Type: core.ConnectionTypeString, Label: "Elapsed Seconds"},
	{Name: "artifacts", Type: core.ConnectionTypeObject, Label: "Artifacts"},
	{Name: "host_status_counts", Type: core.ConnectionTypeObject, Label: "Host Status Counts"},
	{Name: "job_explanation", Type: core.ConnectionTypeString, Label: "Job Explanation"},
	{Name: "result_traceback", Type: core.ConnectionTypeString, Label: "Traceback"},
	{Name: "event_processing_finished", Type: core.ConnectionTypeBoolean, Label: "Events Written"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "Job URL"},
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

	kind := awx.OptionalString("job_kind", inputs)
	path, err := awx.JobKindPath(kind)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if kind == "" {
		kind = awx.JobKindJob
	}

	id, err := awx.RequiredInt("job_id", "Job ID", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	job, err := awx.GetResource(ctx, auth, fmt.Sprintf("%s%d/", path, id), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	out := awx.JobOutputs(auth, kind, job)
	out["id"] = awx.IDString(job["id"])

	// A workflow job is a pure orchestration record: it has no artifacts, no
	// playbook, no host results and no events of its own — everything real lives
	// on its child jobs. Emit nulls rather than fabricating empty objects, so a
	// downstream node can tell "nothing here" from "nothing happened".
	if kind == awx.JobKindWorkflowJob {
		out["artifacts"] = nil
		out["host_status_counts"] = nil
		out["event_processing_finished"] = nil
	}

	status := awx.StringField(job, "status")
	name := awx.StringField(job, "name")
	summary := fmt.Sprintf("%s %d (%s) is %s", label(kind), id, name, status)
	if trace := awx.StringField(job, "result_traceback"); trace != "" {
		summary += " — AWX could not run it; see the traceback"
	}

	out["tool_result"] = summary
	out["success"] = true
	out["error"] = ""
	return out, nil
}

func label(kind string) string {
	switch kind {
	case awx.JobKindWorkflowJob:
		return "Workflow job"
	case awx.JobKindAdHocCommand:
		return "Ad-hoc command"
	case awx.JobKindProjectUpdate:
		return "Project sync"
	case awx.JobKindInventoryUpdate:
		return "Inventory sync"
	default:
		return "Job"
	}
}
