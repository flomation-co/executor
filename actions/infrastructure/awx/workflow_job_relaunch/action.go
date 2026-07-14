// Package infrastructure_awx_workflow_job_relaunch re-runs an AWX / AAP workflow
// job exactly as it ran before.
//
// Two things differ from relaunching a plain job:
//
//   - The body is ignored. There is no `hosts` option (that is jobs-only), so
//     nothing is sent.
//   - Unlike a WORKFLOW TEMPLATE launch, the 201 carries NO "workflow_job" key —
//     the new id is in "id". awx.LaunchedJob absorbs both shapes.
//
// AWX refuses to relaunch a workflow job that was created by slicing a job
// template and has since been orphaned from it, with a 400 whose detail says so.
package infrastructure_awx_workflow_job_relaunch

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Relaunch Workflow Job"
	Description  = "Re-run a workflow job exactly as it ran before."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+rotate-right"
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

	{Name: "workflow_job_id", Type: core.ConnectionTypeString, Label: "Workflow Job ID", Placeholder: "The AWX workflow job to re-run, e.g. 42 — a NEW workflow job is created", Required: true},
	{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Label: "Wait for Completion", Placeholder: "Hold the flow until the new workflow job finishes, then return its result"},
	{Name: "poll_interval_seconds", Type: core.ConnectionTypeInteger, Label: "Poll Interval (seconds)", Placeholder: "How often to check the workflow — default 5s", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "Give up waiting after this long — default 600s (max 3600)", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "ignore_job_failure", Type: core.ConnectionTypeBoolean, Label: "Ignore Workflow Failure", Placeholder: "By default the node fails when the AWX workflow ends failed/error/canceled. Tick to succeed regardless.", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
}

var Outputs = [...]core.Connection{
	{Name: "workflow_job_id", Type: core.ConnectionTypeString, Label: "New Workflow Job ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "elapsed", Type: core.ConnectionTypeString, Label: "Elapsed (seconds)"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "Workflow Job URL"},
	{Name: "timed_out", Type: core.ConnectionTypeBoolean, Label: "Timed Out"},
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

	sourceID, err := awx.RequiredInt("workflow_job_id", "Workflow Job ID", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// No body: a workflow relaunch takes none, and AWX ignores anything sent.
	relaunched, err := awx.CreateResource(ctx, auth, fmt.Sprintf("workflow_jobs/%d/relaunch/", sourceID), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// The new id is in "id", not in a "workflow_job" key — the one place a launch
	// and a relaunch disagree.
	jobID, kind, err := awx.LaunchedJob(relaunched, awx.JobKindWorkflowJob)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	out := map[string]interface{}{
		"workflow_job_id": awx.IDString(jobID),
		"status":          awx.StringField(relaunched, "status"),
		"finished":        false,
		"failed":          false,
		"elapsed":         "",
		"job_url":         awx.JobURL(auth, kind, jobID),
		"timed_out":       false,
		"result":          relaunched,
	}

	if !awx.BoolInput("wait_for_completion", inputs) {
		return success(out, fmt.Sprintf("Relaunched workflow job %d as workflow job %d — it is running in AWX.", sourceID, jobID)), nil
	}

	timeout, _ := awx.OptionalInt("timeout_seconds", inputs)
	poll, _ := awx.OptionalInt("poll_interval_seconds", inputs)

	res, err := awx.WaitForJob(ctx, auth, kind, jobID, awx.WaitOpts{
		PollIntervalSeconds: poll,
		TimeoutSeconds:      timeout,
	})
	if err != nil {
		return failure(out, err.Error()), nil
	}

	job := res.Job
	out["result"] = job
	out["status"] = awx.StringField(job, "status")
	out["finished"] = !res.TimedOut
	out["failed"] = awx.BoolField(job, "failed")
	out["elapsed"] = awx.IDString(job["elapsed"])
	out["timed_out"] = res.TimedOut

	if res.TimedOut {
		return failure(out, fmt.Sprintf(
			"Timed out after %d seconds waiting for workflow job %d (status %q). It is still running in AWX — open %s to watch it.",
			awx.ClampWaitSeconds(timeout), jobID, awx.StringField(job, "status"), awx.JobURL(auth, kind, jobID))), nil
	}

	status := awx.StringField(job, "status")
	if awx.BoolField(job, "failed") && !awx.BoolInput("ignore_job_failure", inputs) {
		msg := fmt.Sprintf("Workflow job %d finished with status %q. Open %s to see which step failed, or use List Workflow Job Nodes to get every step's status.",
			jobID, status, awx.JobURL(auth, kind, jobID))
		if explanation := awx.StringField(job, "job_explanation"); explanation != "" {
			msg += " AWX said: " + explanation
		}
		return failure(out, msg), nil
	}

	return success(out, fmt.Sprintf("Workflow job %d finished with status %q after %ss.", jobID, status, awx.IDString(job["elapsed"]))), nil
}

func success(out map[string]interface{}, summary string) map[string]interface{} {
	out["tool_result"] = summary
	out["success"] = true
	out["error"] = ""
	return out
}

// failure keeps the new workflow job's id in the outputs — it IS running, and the
// operator needs its number — and marks the node failed. Returned with a NIL
// error, so the flow keeps walking.
func failure(out map[string]interface{}, msg string) map[string]interface{} {
	out["tool_result"] = msg
	out["success"] = false
	out["error"] = msg
	return out
}
