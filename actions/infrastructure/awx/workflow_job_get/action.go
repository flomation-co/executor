// Package infrastructure_awx_workflow_job_get fetches one AWX / AAP workflow job.
//
// This is also the endpoint you land on after launching a SLICED job template: a
// job template with job_slice_count > 1 answers its launch with a WORKFLOW job,
// not a job, and polling /jobs/{id}/ for that id gives a 404 nobody can explain.
// is_sliced_job says which of the two you are looking at.
package infrastructure_awx_workflow_job_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Workflow Job"
	Description  = "Fetch one running or finished workflow job — its overall status and timings."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+eye"
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

	{Name: "workflow_job_id", Type: core.ConnectionTypeString, Label: "Workflow Job ID", Placeholder: "The AWX workflow job number, e.g. 42 — from a Launch Workflow Template node or the AWX trigger", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Workflow Job ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "elapsed", Type: core.ConnectionTypeString, Label: "Elapsed (seconds)"},
	{Name: "is_sliced_job", Type: core.ConnectionTypeBoolean, Label: "Is a Sliced Job"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "Workflow Job URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Workflow Job"},
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

	job, err := awx.GetResource(ctx, auth, fmt.Sprintf("workflow_jobs/%d/", id), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	status := awx.StringField(job, "status")
	finished := awx.StringField(job, "finished") != ""

	state := "is still running"
	if finished {
		state = "finished"
	}
	summary := fmt.Sprintf("Workflow job %d %s with status %q", id, state, status)

	out := awx.ObjectResult(job, summary)
	out["status"] = status
	out["finished"] = finished
	out["failed"] = awx.BoolField(job, "failed")
	out["elapsed"] = awx.IDString(job["elapsed"])
	// A sliced JOB TEMPLATE launch produces one of these rather than a job. It is
	// the only way to tell the two apart from the record alone.
	out["is_sliced_job"] = awx.BoolField(job, "is_sliced_job")
	out["job_url"] = awx.JobURL(auth, awx.JobKindWorkflowJob, id)
	return out, nil
}
