// Package infrastructure_awx_project_sync pulls the latest playbooks from a
// project's source control into AWX.
//
// Two AWX quirks shape this action:
//
//   - The POST answers 202 ACCEPTED (not 201) and reports the new project-update
//     id at BOTH .project_update and .id. awx.LaunchedJob absorbs that.
//   - A project that cannot be synced answers 405 METHOD NOT ALLOWED, not 400.
//     can_update is simply bool(scm_type), so a MANUAL project can NEVER be
//     synced — there is no source control to pull from. We GET the same URL
//     first and tell the operator that in plain English rather than surfacing a
//     bare "405".
package infrastructure_awx_project_sync

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Sync Project"
	Description  = "Pull the latest playbooks from a project's source control into AWX. Run this before launching a job template if the playbook has just changed."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+rotate-right"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction

	// Defaults live in Go, not in the Inputs literal: the manifest does not
	// harvest Value, so a default there is invisible to the editor. They are
	// restated in each Placeholder so the operator can actually see them.
	defaultPollSeconds    = 3
	defaultTimeoutSeconds = 300
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

	// --- the project to sync ---
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "Choose the project to sync, or enter its AWX ID", Required: true},

	// --- optionally hold the flow until the sync finishes ---
	{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Label: "Wait For Completion", Placeholder: "Hold the flow until the sync has finished, then report its status"},
	{Name: "poll_interval_seconds", Type: core.ConnectionTypeInteger, Label: "Poll Interval (seconds)", Placeholder: "How often to check the sync — default 3s", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "Give up waiting after this long — default 300s (max 3600)", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "ignore_job_failure", Type: core.ConnectionTypeBoolean, Label: "Ignore Sync Failure", Placeholder: "By default the node fails when the sync ends failed/error/canceled. Tick to succeed regardless.", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "include_stdout", Type: core.ConnectionTypeBoolean, Label: "Include Output", Placeholder: "Return the sync's log — useful when a clone or checkout fails", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
}

var Outputs = [...]core.Connection{
	{Name: "project_update_id", Type: core.ConnectionTypeString, Label: "Project Update ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "scm_revision", Type: core.ConnectionTypeString, Label: "Source Control Revision"},
	{Name: "stdout", Type: core.ConnectionTypeString, Label: "Output"},
	{Name: "elapsed", Type: core.ConnectionTypeString, Label: "Elapsed (seconds)"},
	{Name: "timed_out", Type: core.ConnectionTypeBoolean, Label: "Timed Out"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Project Update"},
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

	projectID, err := awx.RequiredInt("project_id", "Project", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// Pre-flight. can_update is bool(scm_type), so this is the difference between
	// a clear "there is nothing to sync" and an unexplained 405 after the POST.
	preflight, err := awx.GetResource(ctx, auth, fmt.Sprintf("projects/%d/update/", projectID), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if !awx.BoolField(preflight, "can_update") {
		return awx.ErrorResult(cannotSyncMessage(ctx, auth, projectID)), nil
	}

	launched, err := awx.CreateResource(ctx, auth, fmt.Sprintf("projects/%d/update/", projectID), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	updateID, _, err := awx.LaunchedJob(launched, awx.JobKindProjectUpdate)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	if !awx.BoolInput("wait_for_completion", inputs) {
		out := syncOutputs(launched, "", false, false)
		out["project_update_id"] = awx.IDString(updateID)
		out["tool_result"] = fmt.Sprintf("Started a source-control sync of project %d (project update %d)", projectID, updateID)
		out["success"] = true
		out["error"] = ""
		return out, nil
	}

	poll, ok := awx.OptionalInt("poll_interval_seconds", inputs)
	if !ok || poll <= 0 {
		poll = defaultPollSeconds
	}
	timeout, ok := awx.OptionalInt("timeout_seconds", inputs)
	if !ok || timeout <= 0 {
		timeout = defaultTimeoutSeconds
	}
	includeStdout := awx.BoolInput("include_stdout", inputs)

	res, err := awx.WaitForJob(ctx, auth, awx.JobKindProjectUpdate, updateID, awx.WaitOpts{
		PollIntervalSeconds: poll,
		TimeoutSeconds:      timeout,
		IncludeStdout:       includeStdout,
		WaitForEvents:       includeStdout,
	})
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	out := syncOutputs(res.Job, res.Stdout, !res.TimedOut, res.TimedOut)
	out["project_update_id"] = awx.IDString(updateID)

	status := awx.StringField(res.Job, "status")
	switch {
	case res.TimedOut:
		msg := fmt.Sprintf("Project %d is still syncing after %ds (project update %d, status %q). It is still running in AWX.",
			projectID, awx.ClampWaitSeconds(timeout), updateID, status)
		out["tool_result"] = msg
		out["success"] = false
		out["error"] = msg

	case syncFailed(res.Job) && !awx.BoolInput("ignore_job_failure", inputs):
		msg := fmt.Sprintf("The source-control sync of project %d ended %q (project update %d). %s",
			projectID, status, updateID, awx.StringField(res.Job, "job_explanation"))
		out["tool_result"] = msg
		out["success"] = false
		out["error"] = msg

	default:
		out["tool_result"] = fmt.Sprintf("Synced project %d — status %q, revision %s",
			projectID, status, revisionOrNone(res.Job))
		out["success"] = true
		out["error"] = ""
	}
	return out, nil
}

// syncOutputs shapes a project-update record into this action's declared outputs.
func syncOutputs(job map[string]interface{}, stdout string, finished, timedOut bool) map[string]interface{} {
	if job == nil {
		job = map[string]interface{}{}
	}
	return map[string]interface{}{
		"project_update_id": awx.IDString(job["id"]),
		"status":            awx.StringField(job, "status"),
		"finished":          finished,
		"failed":            awx.BoolField(job, "failed"),
		"scm_revision":      awx.StringField(job, "scm_revision"),
		"stdout":            stdout,
		"elapsed":           awx.IDString(job["elapsed"]),
		"timed_out":         timedOut,
		"result":            job,
	}
}

// syncFailed reports whether the sync ended badly. AWX sets `failed` for both a
// failed and an errored update, and a CANCELED update sets neither — so the
// status is checked too.
func syncFailed(job map[string]interface{}) bool {
	if awx.BoolField(job, "failed") {
		return true
	}
	switch awx.StringField(job, "status") {
	case "failed", "error", "canceled":
		return true
	}
	return false
}

func revisionOrNone(job map[string]interface{}) string {
	if rev := awx.StringField(job, "scm_revision"); rev != "" {
		return rev
	}
	return "(none reported)"
}

// cannotSyncMessage explains WHY a project refuses to sync. can_update is
// bool(scm_type), so an empty scm_type — a MANUAL project, whose playbooks were
// copied onto the AWX box by hand — is the overwhelmingly common cause. The
// project fetch is best-effort: if it fails we still return a useful message
// rather than swapping one confusing error for another.
func cannotSyncMessage(ctx context.Context, auth awx.Auth, projectID int64) string {
	project, err := awx.GetResource(ctx, auth, fmt.Sprintf("projects/%d/", projectID), nil)
	if err == nil && awx.StringField(project, "scm_type") == "" {
		return fmt.Sprintf("Project %d is a MANUAL project — its playbooks live on the AWX server itself and there is no source control to pull from, so there is nothing to sync. "+
			"If you meant to sync from Git, set a Source Control Type and URL on the project in AWX.", projectID)
	}
	return fmt.Sprintf("AWX will not sync project %d right now (can_update is false). A manual project has nothing to sync; otherwise check the project has a source-control URL and that a sync is not already running.", projectID)
}
