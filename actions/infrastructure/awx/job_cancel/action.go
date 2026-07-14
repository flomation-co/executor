// Package infrastructure_awx_job_cancel cancels a running AWX job, workflow job
// or ad-hoc command.
package infrastructure_awx_job_cancel

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Cancel Job"
	Description  = "Cancel a running AWX job, workflow job or ad-hoc command. Stops the playbook mid-flight — hosts may be left partially configured."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+circle-stop"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

// defaultCancelWaitSeconds is what Wait Until Canceled uses when Timeout is left
// blank. The manifest does not harvest a default Value, so the default lives here
// and is stated in the placeholder.
const defaultCancelWaitSeconds = 120

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
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Placeholder: "42 — the AWX job ID to cancel", Required: true},
	{Name: "wait_for_cancel", Type: core.ConnectionTypeBoolean, Label: "Wait Until Canceled", Placeholder: "Wait until AWX reports the job as canceled (cancellation is asynchronous)"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "How long to wait for the job to stop — default 120", Visible: &core.VisibleWhen{Field: "wait_for_cancel", Values: []string{"true"}}},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes AWX state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "was_cancellable", Type: core.ConnectionTypeBoolean, Label: "Was Cancellable"},
	{Name: "already_finished", Type: core.ConnectionTypeBoolean, Label: "Already Finished"},
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
		return awx.ErrorResult(fmt.Sprintf("Job Type %q cannot be cancelled — choose Job, Workflow Job or Ad-Hoc Command.", kind)), nil
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

	if err := awx.ConfirmDestructive(inputs, fmt.Sprintf("cancel %s %d", label(kind), id)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// GET /cancel/ answers {"can_cancel": bool}. It is INHERENTLY RACY — the job
	// can go terminal between this read and the POST below — so it is reported to
	// the operator, never trusted as a gate. The 405 handling is what is load-
	// bearing.
	canCancel := false
	if probe, err := awx.GetResource(ctx, auth, fmt.Sprintf("%s%d/cancel/", path, id), nil); err != nil {
		return awx.ErrorResult(err.Error()), nil
	} else {
		canCancel = awx.BoolField(probe, "can_cancel")
	}

	// ★ POSTing /cancel/ on an ALREADY-FINISHED job answers 405 Method Not Allowed
	// — not 409, not 400. That is "already terminal", not a routing bug, and
	// CancelJob reports it as alreadyFinished with a nil error. A success answers
	// 202 with a COMPLETELY EMPTY body, so there is nothing to parse off it.
	alreadyFinished, err := awx.CancelJob(ctx, auth, kind, id)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// Cancellation is ASYNCHRONOUS: cancel() only sets cancel_flag and notifies the
	// dispatcher, so the status can still read "running" for seconds afterwards —
	// and the job can still land on successful if it finished in the race.
	status := ""
	timedOut := false
	if !alreadyFinished && awx.BoolInput("wait_for_cancel", inputs) {
		seconds, set := awx.OptionalInt("timeout_seconds", inputs)
		if !set || seconds <= 0 {
			seconds = defaultCancelWaitSeconds
		}
		res, err := awx.WaitForJob(ctx, auth, kind, id, awx.WaitOpts{
			PollIntervalSeconds: 2,
			TimeoutSeconds:      seconds,
		})
		if err != nil {
			return awx.ErrorResult(err.Error()), nil
		}
		status = awx.StringField(res.Job, "status")
		timedOut = res.TimedOut
	} else {
		job, err := awx.GetResource(ctx, auth, fmt.Sprintf("%s%d/", path, id), nil)
		if err != nil {
			return awx.ErrorResult(err.Error()), nil
		}
		status = awx.StringField(job, "status")
	}

	summary := fmt.Sprintf("Asked AWX to cancel %s %d — it is now %s. Cancellation is asynchronous, so the job may take a few seconds to stop.", label(kind), id, status)
	switch {
	case alreadyFinished:
		summary = fmt.Sprintf("%s %d had already finished (%s), so there was nothing to cancel.", capitalise(label(kind)), id, status)
	case timedOut:
		summary = fmt.Sprintf("Asked AWX to cancel %s %d, but it was still %s when the wait ran out. Check it in AWX.", label(kind), id, status)
	case status == "canceled":
		summary = fmt.Sprintf("%s %d is canceled.", capitalise(label(kind)), id)
	}

	return map[string]interface{}{
		"job_id":           awx.IDString(id),
		"status":           status,
		"was_cancellable":  canCancel,
		"already_finished": alreadyFinished,
		"tool_result":      summary,
		"success":          true,
		"error":            "",
	}, nil
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
