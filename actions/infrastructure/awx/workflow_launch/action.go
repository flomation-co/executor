// Package infrastructure_awx_workflow_launch launches an AWX / AAP workflow job
// template, optionally holding the flow until the whole workflow finishes.
//
// It is a SEPARATE action from job_template_launch rather than a job_kind flag on
// it, because a workflow job template can only ever prompt for SEVEN things —
// inventory, limit, scm_branch, labels, job_tags, skip_tags and extra_vars (they
// all come from SurveyJobTemplateMixin). There is no ask_credential,
// ask_verbosity, ask_job_type, ask_forks, ask_timeout, ask_execution_environment,
// ask_instance_groups, ask_diff_mode or ask_job_slice_count on a workflow — those
// are JobTemplate-only. Sharing one action would show the operator ten fields that
// silently do nothing.
//
// ★ A workflow launch does NOT silently drop a field it does not prompt for —
// this is where it differs from a job template, and it was verified against a live
// AWX 24.6.1. WorkflowJobLaunchSerializer does not pass _exclude_errors=['prompts'],
// so AWX answers 400 — {"limit": ["Field is not configured to prompt on launch."]}
// — and runs nothing. Two consequences:
//
//   - There is NO "Allow Ignored Fields" escape hatch on this action, unlike the
//     job template launch. It could not do anything: AWX will not take the launch
//     however nicely we ask. awx.ValidateLaunch refuses first, with a message that
//     says so.
//   - ignored_fields is therefore ALWAYS EMPTY on a workflow launch. It is still
//     emitted (and still re-checked off the 201) for shape-compatibility with the
//     job template launch, not because AWX ever populates it.
//
// A field outside the seven a workflow can prompt for (verbosity, credentials,
// forks, job_type — all JobTemplate-only) is a third case again: AWX takes the
// launch, drops the field, and does NOT even record it in ignored_fields. Nothing
// this action sends can land there — but Additional Fields can, so a power user
// smuggling verbosity onto a workflow will find it silently ignored by AWX.
//
// A workflow job is a pure orchestration record: it has no artifacts, no playbook,
// no stdout endpoint and no host_status_counts. Everything real lives on the child
// jobs, which is what Include Node Results fetches.
package infrastructure_awx_workflow_launch

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Launch Workflow Template"
	Description  = "Launch an existing AWX / AAP workflow template. Optionally wait for the whole workflow to finish and return every node's result."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+play"
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

	{Name: "workflow_template_id", Type: core.ConnectionTypeString, Label: "Workflow Template", Placeholder: "Pick the workflow template to launch", Required: true},
	{Name: "extra_vars", Type: core.ConnectionTypeObject, Label: "Extra Variables / Survey Answers", Placeholder: `{"target_env":"prod"} — survey answers go here too, keyed by the survey variable name`},

	{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Label: "Wait for Completion", Placeholder: "Hold the flow until the whole workflow finishes, then return its result"},
	{Name: "poll_interval_seconds", Type: core.ConnectionTypeInteger, Label: "Poll Interval (seconds)", Placeholder: "How often to check the workflow — default 5s", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "Give up waiting after this long — default 600s (max 3600)", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "cancel_on_timeout", Type: core.ConnectionTypeBoolean, Label: "Cancel on Timeout", Placeholder: "Cancel the workflow in AWX if the wait runs out. Leave unticked to let it carry on running.", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "ignore_job_failure", Type: core.ConnectionTypeBoolean, Label: "Ignore Workflow Failure", Placeholder: "By default the node fails when the AWX workflow ends failed/error/canceled. Tick to succeed regardless.", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "include_node_results", Type: core.ConnectionTypeBoolean, Label: "Include Node Results", Placeholder: "Also return every workflow node and its child job status", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Only if the workflow prompts for an inventory — otherwise leave blank"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "web*:&prod — Ansible host pattern. Only if the workflow prompts for it."},
	{Name: "scm_branch", Type: core.ConnectionTypeString, Label: "Source Control Branch", Placeholder: "Only if the workflow prompts for a branch"},
	{Name: "job_tags", Type: core.ConnectionTypeString, Label: "Job Tags", Placeholder: "Comma-separated Ansible tags to run — only if the workflow prompts for them"},
	{Name: "skip_tags", Type: core.ConnectionTypeString, Label: "Skip Tags", Placeholder: "Comma-separated Ansible tags to skip — only if the workflow prompts for them"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated label IDs, e.g. 2,5 — only if the workflow prompts for labels"},
	// No "Allow Ignored Fields" here, unlike the job template launch: AWX REFUSES a
	// workflow launch that sets a non-prompted field (400) rather than silently
	// dropping it, so there is nothing an escape hatch could unlock.
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"scm_branch":"main"} — any other launch field; overrides the fields above`},
}

var Outputs = [...]core.Connection{
	{Name: "workflow_job_id", Type: core.ConnectionTypeString, Label: "Workflow Job ID"},
	// Always "workflow_job". Emitted so the Job Type of a downstream Get Job /
	// Wait for Job / Cancel Job node can be WIRED from this one: those actions
	// default to a plain job, and /jobs/{id}/ 404s for a workflow job id.
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Type"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "elapsed", Type: core.ConnectionTypeString, Label: "Elapsed (seconds)"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "Workflow Job URL"},
	{Name: "ignored_fields", Type: core.ConnectionTypeObject, Label: "Ignored Fields"},
	{Name: "node_results", Type: core.ConnectionTypeObject, Label: "Node Results"},
	{Name: "timed_out", Type: core.ConnectionTypeBoolean, Label: "Timed Out"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Workflow Job"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		// The ONE hard failure: the node is mis-configured, not the request.
		return nil, err
	}

	ctx, cancel := awx.Context()
	defer cancel()

	templateID, err := awx.RequiredInt("workflow_template_id", "Workflow Template", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	body, err := launchBody(inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// ★ The safety property of this node. Refuses, before anything runs, any
	// override this workflow is not configured to accept. Always fails closed —
	// AWX would answer 400 and run nothing anyway, so there is no "send it anyway"
	// to offer (see the package comment).
	if _, err := awx.ValidateLaunch(ctx, auth, awx.TemplateKindWorkflow, templateID, body, false); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	launched, err := awx.CreateResource(ctx, auth, fmt.Sprintf("workflow_job_templates/%d/launch/", templateID), body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	jobID, kind, err := awx.LaunchedJob(launched, awx.JobKindWorkflowJob)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	out := map[string]interface{}{
		"workflow_job_id": awx.IDString(jobID),
		"job_kind":        kind,
		"status":          awx.StringField(launched, "status"),
		"finished":        false,
		"failed":          false,
		"elapsed":         "",
		"job_url":         awx.JobURL(auth, kind, jobID),
		"ignored_fields":  map[string]interface{}{},
		"node_results":    []interface{}{},
		"timed_out":       false,
		"result":          launched,
	}

	// Belt and braces: the workflow could have been reconfigured between the
	// pre-flight and the launch, so the 201 is re-checked. In practice AWX never
	// populates ignored_fields on a workflow launch — it 400s instead — so this is
	// shape-compatibility with the job template launch, not a live code path.
	ignored, ignoredErr := awx.CheckIgnoredFields(launched, false)
	if ignored == nil {
		ignored = map[string]interface{}{}
	}
	out["ignored_fields"] = ignored
	if ignoredErr != nil {
		return failure(out, ignoredErr.Error()), nil
	}

	if !awx.BoolInput("wait_for_completion", inputs) {
		return success(out, fmt.Sprintf("Launched workflow job %d — it is running in AWX. Tick Wait for Completion to hold the flow until it finishes.", jobID)), nil
	}

	timeout, _ := awx.OptionalInt("timeout_seconds", inputs)
	poll, _ := awx.OptionalInt("poll_interval_seconds", inputs)

	res, err := awx.WaitForJob(ctx, auth, kind, jobID, awx.WaitOpts{
		PollIntervalSeconds: poll,
		TimeoutSeconds:      timeout,
		CancelOnTimeout:     awx.BoolInput("cancel_on_timeout", inputs),
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

	// Fetched even on a timeout: which step a stuck workflow is sitting on is
	// exactly what the operator needs to see.
	if awx.BoolInput("include_node_results", inputs) {
		nodes, err := fetchNodeResults(ctx, auth, jobID)
		if err != nil {
			return failure(out, err.Error()), nil
		}
		out["node_results"] = nodes
	}

	if res.TimedOut {
		msg := fmt.Sprintf("Timed out after %d seconds waiting for workflow job %d (status %q).",
			awx.ClampWaitSeconds(timeout), jobID, awx.StringField(job, "status"))
		if res.Canceled {
			msg += " It has been cancelled in AWX, as Cancel on Timeout is ticked."
		} else {
			msg += " It is still running in AWX — open " + awx.JobURL(auth, kind, jobID) + " to watch it, or use Wait for Job with a longer timeout."
		}
		return failure(out, msg), nil
	}

	status := awx.StringField(job, "status")
	if awx.BoolField(job, "failed") && !awx.BoolInput("ignore_job_failure", inputs) {
		msg := fmt.Sprintf("Workflow job %d finished with status %q. Open %s to see which step failed, or tick Include Node Results to get every step's status back on this node.",
			jobID, status, awx.JobURL(auth, kind, jobID))
		if explanation := awx.StringField(job, "job_explanation"); explanation != "" {
			msg += " AWX said: " + explanation
		}
		return failure(out, msg), nil
	}

	return success(out, fmt.Sprintf("Workflow job %d finished with status %q after %ss.", jobID, status, awx.IDString(job["elapsed"]))), nil
}

// launchBody assembles the launch payload. Only the seven fields a workflow job
// template can prompt for are ever sent; Additional Fields is merged LAST so a
// power user's key overrides a first-class input, and so anything smuggled in
// there still goes through the ignored-fields pre-flight.
func launchBody(inputs []*core.Connection) (map[string]interface{}, error) {
	body := map[string]interface{}{}

	extraVars, err := awx.OptionalJSONObject("extra_vars", inputs)
	if err != nil {
		return nil, err
	}
	if len(extraVars) > 0 {
		body["extra_vars"] = extraVars
	}

	if err := awx.SetIntIfPresent(body, inputs, "inventory", "inventory_id"); err != nil {
		return nil, err
	}
	awx.SetIfPresent(body, inputs, "limit", "limit")
	awx.SetIfPresent(body, inputs, "scm_branch", "scm_branch")
	awx.SetIfPresent(body, inputs, "job_tags", "job_tags")
	awx.SetIfPresent(body, inputs, "skip_tags", "skip_tags")
	awx.SetIntListIfPresent(body, inputs, "labels", "labels")

	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	return body, nil
}

// fetchNodeResults walks the workflow job's nodes and reduces each to its child
// job's status. summary_fields.job carries {id,name,status,failed,elapsed,type}
// already, so one call gives the whole DAG with no N+1.
func fetchNodeResults(ctx context.Context, auth awx.Auth, jobID int64) ([]interface{}, error) {
	items, _, _, err := awx.List(ctx, auth, fmt.Sprintf("workflow_jobs/%d/workflow_nodes/", jobID), nil, true)
	if err != nil {
		return nil, err
	}
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		if summary := summariseNode(item); summary != nil {
			out = append(out, summary)
		}
	}
	return out, nil
}

// summariseNode flattens one workflow job node.
//
// node.job is null until the child job is created — and stays null FOREVER when
// do_not_run is true (the branch was not taken). That is not a failure, so it is
// surfaced as do_not_run rather than as a missing job.
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
	// job|workflow_approval|project_update|inventory_update|workflow_job — a
	// "stuck" workflow is usually one sitting on a workflow_approval.
	out["job_type"] = awx.StringField(job, "type")
	out["status"] = awx.StringField(job, "status")
	out["failed"] = awx.BoolField(job, "failed")
	out["elapsed"] = awx.IDString(job["elapsed"])
	return out
}

func success(out map[string]interface{}, summary string) map[string]interface{} {
	out["tool_result"] = summary
	out["success"] = true
	out["error"] = ""
	return out
}

// failure keeps every output the node has already established (the workflow job
// id above all — the job IS running, and the operator needs its number) and marks
// the node failed. Returned with a NIL error, so the flow keeps walking.
func failure(out map[string]interface{}, msg string) map[string]interface{} {
	out["tool_result"] = msg
	out["success"] = false
	out["error"] = msg
	return out
}
