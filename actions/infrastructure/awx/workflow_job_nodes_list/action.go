// Package infrastructure_awx_workflow_job_nodes_list lists every step of an AWX /
// AAP workflow job, with the status of the job each step ran.
//
// This is THE endpoint for workflow results. Each node's summary_fields.job
// carries {id, name, status, failed, elapsed, type}, so one call gives the whole
// DAG plus every child job's status — no N+1 walk.
//
// Two shapes an operator has to be protected from:
//
//   - node.job is null until the child job is created, and stays null FOREVER when
//     do_not_run is true (the branch was not taken). A not-taken branch is NOT a
//     failure, and is never reported as one.
//   - summary_fields.job.type is one of job, workflow_approval, project_update,
//     inventory_update or a nested workflow_job. The type is surfaced on every
//     node, because a workflow that looks "stuck" is nearly always one sitting on
//     a workflow_approval waiting for a human.
package infrastructure_awx_workflow_job_nodes_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: List Workflow Job Nodes"
	Description  = "List every step of a running or finished workflow, with the status of the job each step ran. This is how you find which part of a workflow failed."
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

	{Name: "workflow_job_id", Type: core.ConnectionTypeString, Label: "Workflow Job ID", Placeholder: "The AWX workflow job number, e.g. 42", Required: true},
	{Name: "failed_only", Type: core.ConnectionTypeBoolean, Label: "Failed Steps Only", Placeholder: "Return only the steps whose job failed, errored or was cancelled"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1-based page number — default 1"},
	{Name: "page_size", Type: core.ConnectionTypeInteger, Label: "Page Size", Placeholder: "Steps per page — default 50, max 200"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow every page until all steps are fetched"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Steps"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_count", Type: core.ConnectionTypeInteger, Label: "Total Matching"},
	{Name: "failed_nodes", Type: core.ConnectionTypeObject, Label: "Failed Steps"},
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

	id, err := awx.RequiredInt("workflow_job_id", "Workflow Job ID", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// No order_by: a workflow's nodes are a DAG, and AWX's own id order is the
	// closest thing to the order they were created in.
	q, returnAll := awx.ListParams(inputs, "")
	items, total, hasMore, err := awx.List(ctx, auth, fmt.Sprintf("workflow_jobs/%d/workflow_nodes/", id), q, returnAll)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// Failed steps are computed client-side rather than filtered server-side: what
	// counts as failed lives on the CHILD job (summary_fields.job.failed / .status),
	// and a node whose branch was never taken has no child job at all.
	failed := []interface{}{}
	pendingApprovals := 0
	for _, item := range items {
		summary := summariseNode(item)
		if summary == nil {
			continue
		}
		if nodeFailed(summary) {
			failed = append(failed, summary)
		}
		if summary["job_type"] == "workflow_approval" && summary["status"] == "pending" {
			pendingApprovals++
		}
	}

	results := items
	if awx.BoolInput("failed_only", inputs) {
		results = failed
	}

	summary := fmt.Sprintf("Workflow job %d has %d step(s), %d of which failed", id, len(items), len(failed))
	if awx.BoolInput("failed_only", inputs) {
		summary = fmt.Sprintf("Workflow job %d has %d failed step(s) of %d", id, len(failed), len(items))
	}
	if pendingApprovals > 0 {
		summary += fmt.Sprintf(". %d step(s) are waiting for a human to approve them in AWX — the workflow is not stuck, it is paused", pendingApprovals)
	}
	if hasMore {
		summary += " — more steps remain; tick Return All to fetch them"
	}

	out := awx.ListResult(results, total, hasMore, summary)
	out["failed_nodes"] = failed
	return out, nil
}

// summariseNode flattens one workflow job node into the fields that answer "what
// ran here, and how did it go?".
func summariseNode(raw interface{}) map[string]interface{} {
	n, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	out := map[string]interface{}{
		"node_id":    awx.IDString(n["id"]),
		"job_id":     "",
		"job_name":   "",
		"job_type":   "",
		"status":     "",
		"failed":     false,
		"elapsed":    "",
		"do_not_run": awx.BoolField(n, "do_not_run"),
	}

	summaryFields, _ := n["summary_fields"].(map[string]interface{})
	if template, ok := summaryFields["unified_job_template"].(map[string]interface{}); ok {
		// Named even when the step never ran, so a not-taken branch is still
		// identifiable in the output.
		out["job_name"] = awx.StringField(template, "name")
	}

	job, _ := summaryFields["job"].(map[string]interface{})
	if job == nil {
		return out
	}
	out["job_id"] = awx.IDString(job["id"])
	if name := awx.StringField(job, "name"); name != "" {
		out["job_name"] = name
	}
	out["job_type"] = awx.StringField(job, "type")
	out["status"] = awx.StringField(job, "status")
	out["failed"] = awx.BoolField(job, "failed")
	out["elapsed"] = awx.IDString(job["elapsed"])
	return out
}

// nodeFailed reports whether a step actually went wrong.
//
// A node with no child job has NOT failed — either the job has not been created
// yet, or do_not_run is set and the branch was never taken. Reporting a
// not-taken branch as a failure is the classic mis-read of this endpoint.
func nodeFailed(summary map[string]interface{}) bool {
	if summary["job_id"] == "" {
		return false
	}
	if failed, ok := summary["failed"].(bool); ok && failed {
		return true
	}
	switch summary["status"] {
	case "failed", "error", "canceled":
		return true
	}
	return false
}
