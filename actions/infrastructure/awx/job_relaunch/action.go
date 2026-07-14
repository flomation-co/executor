// Package infrastructure_awx_job_relaunch re-runs an AWX job with the settings it
// ran with before.
package infrastructure_awx_job_relaunch

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Relaunch Job"
	Description  = "Re-run an AWX job with the same settings it ran with before — either against all its hosts, or only the hosts that failed."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+rotate-right"
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
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Type", Placeholder: "Which kind of AWX job — a playbook Job unless you know otherwise", Options: []core.ConnectionOption{
		{Name: "Job", Value: "job"},
		{Name: "Workflow Job", Value: "workflow_job"},
		{Name: "Ad-Hoc Command", Value: "ad_hoc_command"},
	}},
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Placeholder: "42 — the AWX job to re-run", Required: true},
	{Name: "hosts", Type: core.ConnectionTypeString, Label: "Hosts", Placeholder: "Which hosts to re-run against — all of them, or only the ones that failed", Options: []core.ConnectionOption{
		{Name: "All hosts", Value: "all"},
		{Name: "Only failed and unreachable hosts", Value: "failed"},
	}, Visible: &core.VisibleWhen{Field: "job_kind", Values: []string{"", "job"}}},
	{Name: "job_type", Type: core.ConnectionTypeString, Label: "Job Type (run or check)", Placeholder: "Leave blank to re-run exactly as before", Options: []core.ConnectionOption{
		{Name: "Run", Value: "run"},
		{Name: "Check (dry run)", Value: "check"},
	}, Visible: &core.VisibleWhen{Field: "job_kind", Values: []string{"", "job"}}},
	{Name: "credential_passwords", Type: core.ConnectionTypeObject, Label: "Credential Passwords", Placeholder: `{"ssh_password": "…"} — only if the job's credentials ask for a password at launch`},
	{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Label: "Wait for Completion", Placeholder: "Hold the flow until the re-run finishes"},
	{Name: "poll_interval_seconds", Type: core.ConnectionTypeInteger, Label: "Poll Interval (seconds)", Placeholder: "How often to check AWX — default 5", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "Give up waiting after this long — default 600, maximum 3600", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "ignore_job_failure", Type: core.ConnectionTypeBoolean, Label: "Ignore Job Failure", Placeholder: "Carry on even if the playbook fails — leave unticked to fail this node when the job fails", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
}

var Outputs = [...]core.Connection{
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "New Job ID"},
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Type"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "retry_counts", Type: core.ConnectionTypeObject, Label: "Retry Counts"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "Job URL"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "elapsed", Type: core.ConnectionTypeString, Label: "Elapsed Seconds"},
	{Name: "artifacts", Type: core.ConnectionTypeObject, Label: "Artifacts"},
	{Name: "host_status_counts", Type: core.ConnectionTypeObject, Label: "Host Status Counts"},
	{Name: "job_explanation", Type: core.ConnectionTypeString, Label: "Job Explanation"},
	{Name: "result_traceback", Type: core.ConnectionTypeString, Label: "Traceback"},
	{Name: "event_processing_finished", Type: core.ConnectionTypeBoolean, Label: "Events Written"},
	{Name: "timed_out", Type: core.ConnectionTypeBoolean, Label: "Timed Out"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Job"},
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
		return awx.ErrorResult(fmt.Sprintf("Job Type %q cannot be relaunched — choose Job, Workflow Job or Ad-Hoc Command. (A project or inventory sync is re-run with Sync Project / Sync Inventory Source.)", kind)), nil
	}
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

	// GET /relaunch/ reports how many hosts are eligible for a retry. It is
	// informational only — and TYPE-UNSTABLE: retry_counts is a DICT once the job
	// has finished but a plain STRING while it is still active, so it is carried as
	// an `any` and never decoded into a map. A failure here is not a reason to
	// refuse the relaunch.
	var retryCounts interface{}
	if probe, err := awx.GetResource(ctx, auth, fmt.Sprintf("%s%d/relaunch/", path, id), nil); err == nil {
		retryCounts = probe["retry_counts"]
	}

	// Relaunch accepts ONLY {hosts, job_type, credential_passwords}. It re-runs the
	// SAVED launch configuration — new extra_vars / limit / inventory / verbosity
	// cannot be passed here; launching the template again is the way to change them.
	body := map[string]interface{}{}
	if kind == awx.JobKindJob {
		awx.SetIfPresent(body, inputs, "hosts", "hosts")
		awx.SetIfPresent(body, inputs, "job_type", "job_type")
	}
	if err := awx.SetJSONIfPresent(body, inputs, "credential_passwords", "credential_passwords"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	launched, err := awx.CreateResource(ctx, auth, fmt.Sprintf("%s%d/relaunch/", path, id), body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// Three different new-id conventions: a job relaunch answers {"job": id}, an
	// ad-hoc relaunch {"ad_hoc_command": id}, and a WORKFLOW relaunch adds no extra
	// key at all — the new id is in "id". LaunchedJob absorbs all three.
	newID, newKind, err := awx.LaunchedJob(launched, kind)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	out := awx.JobOutputs(auth, newKind, launched)
	out["job_id"] = awx.IDString(newID)
	out["job_url"] = awx.JobURL(auth, newKind, newID)
	out["retry_counts"] = retryCounts
	out["timed_out"] = false

	hosts := awx.OptionalString("hosts", inputs)
	scope := ""
	if kind == awx.JobKindJob && hosts == "failed" {
		scope = " against its failed and unreachable hosts only"
	}

	if !awx.BoolInput("wait_for_completion", inputs) {
		out["tool_result"] = fmt.Sprintf("Relaunched %s %d as %s %d%s. It is %s — use Wait for Job to follow it.",
			label(kind), id, label(newKind), newID, scope, awx.StringField(launched, "status"))
		out["success"] = true
		out["error"] = ""
		return out, nil
	}

	poll, _ := awx.OptionalInt("poll_interval_seconds", inputs)
	timeout, _ := awx.OptionalInt("timeout_seconds", inputs)

	res, err := awx.WaitForJob(ctx, auth, newKind, newID, awx.WaitOpts{
		PollIntervalSeconds: poll,
		TimeoutSeconds:      timeout,
		WaitForEvents:       true,
	})
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	out = awx.JobOutputs(auth, newKind, res.Job)
	out["job_id"] = awx.IDString(newID)
	out["retry_counts"] = retryCounts
	out["timed_out"] = res.TimedOut

	status := awx.StringField(res.Job, "status")
	if res.TimedOut {
		msg := fmt.Sprintf("Relaunched %s %d as %s %d, but it was still %s after %d second(s). The job is still running in AWX — find it at %s.",
			label(kind), id, label(newKind), newID, status, awx.ClampWaitSeconds(timeout), awx.JobURL(auth, newKind, newID))
		return fail(out, msg), nil
	}

	if awx.BoolField(res.Job, "failed") && !awx.BoolInput("ignore_job_failure", inputs) {
		msg := fmt.Sprintf("%s %d (relaunched from %d) finished with status %s.", capitalise(label(newKind)), newID, id, status)
		if trace := awx.StringField(res.Job, "result_traceback"); trace != "" {
			msg += " AWX could not run it — see the traceback."
		}
		msg += " Tick “Ignore Job Failure” to carry on anyway."
		return fail(out, msg), nil
	}

	out["tool_result"] = fmt.Sprintf("Relaunched %s %d as %s %d%s — it finished with status %s.",
		label(kind), id, label(newKind), newID, scope, status)
	out["success"] = true
	out["error"] = ""
	return out, nil
}

// fail marks the outputs as a SOFT failure while keeping the job's own outputs
// intact, so a downstream node can still read the id, status and traceback of the
// job that failed. Returned with a nil error — a non-nil error would abort the
// whole flow.
func fail(out map[string]interface{}, msg string) map[string]interface{} {
	out["tool_result"] = msg
	out["success"] = false
	out["error"] = msg
	return out
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

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
