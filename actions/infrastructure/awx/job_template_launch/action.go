// Package infrastructure_awx_job_template_launch launches an AWX / AAP job
// template — the node's primary purpose — and optionally holds the flow until the
// job finishes, returning its status, output and artifacts.
//
// Waiting is a FLAG on this action rather than a separate action: the ~25 inputs
// would otherwise be duplicated (doubling the drift surface and the pre-flight and
// survey logic), and the wait outputs are a strict superset of the async ones, so
// flipping async → sync is one checkbox instead of deleting the node and re-wiring
// every downstream variable reference. (A standalone Wait for Job still exists, for
// waiting on a job someone ELSE launched — one that arrived on the AWX trigger, for
// instance. That is a genuinely different operation.)
//
// Three AWX hazards decide the shape of this file:
//
//   - ★ SILENTLY IGNORED PROMPTS. AWX answers 201 and DROPS any prompt field whose
//     matching ask_*_on_launch flag is off, recording it only in the response's
//     ignored_fields. Sending Limit=web* to a template with ask_limit_on_launch
//     false RUNS THE PLAYBOOK AGAINST EVERY HOST IN THE INVENTORY, and the operator
//     is never told. So the node pre-flights (GET .../launch/) and FAILS CLOSED,
//     naming the offending field, before anything runs — unless the operator has
//     explicitly ticked Allow Ignored Fields. The 201 is then re-checked, because
//     the template can be reconfigured in between. This is the single most valuable
//     safety property of the node.
//
//   - SURVEYS BYPASS ask_variables_on_launch. A survey answer is always accepted;
//     only extra_vars keys the survey does NOT own need that flag. Answers are
//     validated client-side (required / type / min / max / choices) so the operator
//     sees the problem in the editor rather than an opaque AWX 400.
//
//   - ★ THE 201 IS POLYMORPHIC. If the effective job_slice_count is > 1 the launch
//     produces a WORKFLOW JOB — {"workflow_job": 99, "type": "workflow_job"} with no
//     "job" key at all — and polling /jobs/99/ for it 404s on an id that plainly
//     exists, which is un-debuggable from the AWX UI. awx.LaunchedJob branches on
//     the type, and everything downstream follows the kind it reports.
//
// Launching is NOT guarded by Confirm Destructive Action: it is what this node is
// for, and AWX's own Execute-role RBAC is the real gate.
package infrastructure_awx_job_template_launch

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Launch Job Template"
	Description  = "Launch an existing AWX / AAP job template. Optionally hold the flow until the job finishes and return its status, output and artifacts. Survey answers and extra variables go in one place; advanced prompt overrides (inventory, limit, tags, verbosity, branch, credentials) are available when the template allows them."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+play"
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
	{Name: "job_template_id", Type: core.ConnectionTypeString, Label: "Job Template", Placeholder: "Pick the job template to launch", Required: true},
	{Name: "extra_vars", Type: core.ConnectionTypeObject, Label: "Extra Variables / Survey Answers", Placeholder: `{"target_env":"prod"} — survey answers go here too, keyed by the survey variable name`},

	// ---- WAIT ----
	{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Label: "Wait for Completion", Placeholder: "Hold the flow until the job finishes, then return its status, output and artifacts"},
	{Name: "poll_interval_seconds", Type: core.ConnectionTypeInteger, Label: "Poll Interval (seconds)", Placeholder: "How often to check the job — default 5s", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "Give up waiting after this long — default 600s (max 3600)", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "cancel_on_timeout", Type: core.ConnectionTypeBoolean, Label: "Cancel on Timeout", Placeholder: "Cancel the job in AWX if the wait runs out. Leave unticked to let it carry on running.", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "ignore_job_failure", Type: core.ConnectionTypeBoolean, Label: "Ignore Job Failure", Placeholder: "By default the node fails when the AWX job ends failed/error/canceled. Tick to succeed regardless.", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "include_stdout", Type: core.ConnectionTypeBoolean, Label: "Include Output", Placeholder: "Also return the playbook's text output (stdout)", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "stdout_max_bytes", Type: core.ConnectionTypeInteger, Label: "Max Output Bytes", Placeholder: "Truncate the output at this many bytes — default 1048576 (1 MB)", Visible: &core.VisibleWhen{Field: "include_stdout", Values: []string{"true"}}},

	// ---- ADVANCED: prompt overrides. Each is sent only when the template's matching
	// 'Prompt on launch' is on — the node refuses up-front otherwise, rather than
	// letting AWX silently drop it. ----
	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Only if the template prompts for an inventory — otherwise leave blank"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "web*:&prod — Ansible host pattern. Only if the template prompts for it."},
	{Name: "job_tags", Type: core.ConnectionTypeString, Label: "Job Tags", Placeholder: "Comma-separated Ansible tags to run — only if the template prompts for them"},
	{Name: "skip_tags", Type: core.ConnectionTypeString, Label: "Skip Tags", Placeholder: "Comma-separated Ansible tags to skip — only if the template prompts for them"},
	{Name: "job_type", Type: core.ConnectionTypeString, Label: "Job Type", Placeholder: "Only if the template prompts for it", Options: []core.ConnectionOption{
		{Name: "Template default", Value: ""},
		{Name: "Run", Value: "run"},
		{Name: "Check (dry run)", Value: "check"},
	}},
	{Name: "verbosity", Type: core.ConnectionTypeString, Label: "Verbosity", Placeholder: "How much detail the playbook logs — only if the template prompts for it", Options: []core.ConnectionOption{
		{Name: "Template default", Value: ""},
		{Name: "0 — Normal", Value: "0"},
		{Name: "1 — Verbose", Value: "1"},
		{Name: "2 — More Verbose", Value: "2"},
		{Name: "3 — Debug", Value: "3"},
		{Name: "4 — Connection Debug", Value: "4"},
		{Name: "5 — WinRM Debug", Value: "5"},
	}},
	{Name: "scm_branch", Type: core.ConnectionTypeString, Label: "Source Control Branch", Placeholder: "Only if the template prompts for a branch"},
	{Name: "credentials", Type: core.ConnectionTypeString, Label: "Credentials", Placeholder: "Comma-separated credential IDs, e.g. 1,4 — blank uses the template's own"},
	{Name: "credential_passwords", Type: core.ConnectionTypeObject, Label: "Credential Passwords", Placeholder: `{"ssh_password":"…","vault_password.dev":"…"} — only if the template asks for them`},
	{Name: "execution_environment_id", Type: core.ConnectionTypeString, Label: "Execution Environment", Placeholder: "Only if the template prompts for one"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated label IDs, e.g. 2,5 — only if the template prompts for labels"},
	{Name: "instance_groups", Type: core.ConnectionTypeString, Label: "Instance Groups", Placeholder: "Comma-separated instance group IDs — only if the template prompts for them"},
	{Name: "forks", Type: core.ConnectionTypeInteger, Label: "Forks", Placeholder: "How many hosts to configure at once — only if the template prompts for it"},
	{Name: "job_slice_count", Type: core.ConnectionTypeInteger, Label: "Job Slicing", Placeholder: "Split the run across this many jobs — note anything above 1 produces a WORKFLOW job"},
	{Name: "launch_timeout", Type: core.ConnectionTypeInteger, Label: "Job Timeout (seconds)", Placeholder: "AWX-side job timeout in seconds (not the wait timeout) — only if the template prompts for it"},
	{Name: "diff_mode", Type: core.ConnectionTypeBoolean, Label: "Show Changes (diff mode)", Placeholder: "Report what each task changed — only if the template prompts for it"},
	{Name: "allow_ignored_fields", Type: core.ConnectionTypeBoolean, Label: "Allow Ignored Fields", Placeholder: "Send the overrides above even when the template is not configured to accept them. AWX will SILENTLY DROP them and run anyway — leave unticked."},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"diff_mode":false} — any other launch field; overrides the fields above`},
}

var Outputs = [...]core.Connection{
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID"},
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Type"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "Job URL"},
	{Name: "ignored_fields", Type: core.ConnectionTypeObject, Label: "Ignored Fields"},
	{Name: "elapsed", Type: core.ConnectionTypeString, Label: "Elapsed Seconds"},
	{Name: "artifacts", Type: core.ConnectionTypeObject, Label: "Artifacts"},
	{Name: "host_status_counts", Type: core.ConnectionTypeObject, Label: "Host Status Counts"},
	{Name: "job_explanation", Type: core.ConnectionTypeString, Label: "Job Explanation"},
	{Name: "result_traceback", Type: core.ConnectionTypeString, Label: "Traceback"},
	{Name: "event_processing_finished", Type: core.ConnectionTypeBoolean, Label: "Events Written"},
	{Name: "stdout", Type: core.ConnectionTypeString, Label: "Output"},
	{Name: "timed_out", Type: core.ConnectionTypeBoolean, Label: "Timed Out"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Job"},
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

	templateID, err := awx.RequiredInt("job_template_id", "Job Template", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	body, err := launchBody(inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	allowIgnored := awx.BoolInput("allow_ignored_fields", inputs)

	// ★ Fail closed BEFORE anything runs. ValidateLaunch fetches the template's
	// launch configuration, validates the survey answers against its spec, and
	// refuses any override the template is not configured to accept — which AWX
	// would otherwise take, silently drop, and run without.
	cfg, err := awx.ValidateLaunch(ctx, auth, awx.TemplateKindJob, templateID, body, allowIgnored)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	template := templateName(cfg, templateID)

	launched, err := awx.CreateResource(ctx, auth, fmt.Sprintf("job_templates/%d/launch/", templateID), body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// ★ The 201 is polymorphic: a sliced template (job_slice_count > 1) hands back a
	// WORKFLOW job, with no "job" key at all. Everything below follows the kind AWX
	// actually reports, so the wait polls the collection the job really lives in.
	jobID, kind, err := awx.LaunchedJob(launched, awx.JobKindJob)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	out := jobOutputs(auth, kind, jobID, launched)
	out["stdout"] = ""
	out["timed_out"] = false

	// Belt and braces: the template can be reconfigured between the pre-flight and
	// the launch, so the 201 is re-checked. ignored_fields is emitted either way —
	// it is the only trace AWX leaves of a dropped field.
	ignored, ignoredErr := awx.CheckIgnoredFields(launched, allowIgnored)
	if ignored == nil {
		ignored = map[string]interface{}{}
	}
	out["ignored_fields"] = ignored
	if ignoredErr != nil {
		return failure(out, ignoredErr.Error()), nil
	}

	if !awx.BoolInput("wait_for_completion", inputs) {
		return success(out, fmt.Sprintf("Launched %s — AWX %s %d is queued. Tick Wait for Completion to hold the flow until it finishes.",
			template, kindLabel(kind), jobID)), nil
	}

	timeout, _ := awx.OptionalInt("timeout_seconds", inputs)
	poll, _ := awx.OptionalInt("poll_interval_seconds", inputs)
	stdoutMax, _ := awx.OptionalInt("stdout_max_bytes", inputs)

	res, err := awx.WaitForJob(ctx, auth, kind, jobID, awx.WaitOpts{
		PollIntervalSeconds: poll,
		TimeoutSeconds:      timeout,
		CancelOnTimeout:     awx.BoolInput("cancel_on_timeout", inputs),
		IncludeStdout:       awx.BoolInput("include_stdout", inputs),
		StdoutMaxBytes:      stdoutMax,
		// Artifacts and Host Status Counts are outputs of this node, and AWX writes
		// job events ASYNCHRONOUSLY — status flips to successful the instant the
		// runner exits, while the events are still flushing to Postgres. Without
		// this, a set_stats artifact read by the next node in the flow is
		// intermittently empty. It costs nothing in the usual case, where the events
		// are already written by the time the job goes terminal.
		WaitForEvents: true,
	})
	if err != nil {
		return failure(out, err.Error()), nil
	}

	job := res.Job
	for k, v := range jobOutputs(auth, kind, jobID, job) {
		out[k] = v
	}
	out["timed_out"] = res.TimedOut
	out["stdout"] = res.Stdout

	status := awx.StringField(job, "status")
	url := awx.JobURL(auth, kind, jobID)

	if res.TimedOut {
		msg := fmt.Sprintf("Timed out after %d seconds waiting for AWX %s %d (it is %s).",
			awx.ClampWaitSeconds(timeout), kindLabel(kind), jobID, statusOrUnknown(status))
		if res.Canceled {
			msg += " It has been cancelled in AWX, as Cancel on Timeout is ticked."
		} else {
			msg += " It is still running in AWX — open " + url + " to watch it, or raise Timeout (max 3600 seconds)."
		}
		return failure(out, msg), nil
	}

	if jobEndedBadly(status, awx.BoolField(job, "failed")) && !awx.BoolInput("ignore_job_failure", inputs) {
		msg := fmt.Sprintf("AWX %s %d (%s) finished with status %q. Open %s to see what failed.",
			kindLabel(kind), jobID, template, status, url)
		// Only an ERRORED job has a traceback: AWX could not run the playbook at
		// all. A FAILED one ran fine and the hosts failed — a different problem.
		if trace := awx.StringField(job, "result_traceback"); trace != "" {
			msg += " AWX could not run the playbook — see the Traceback output."
		}
		if explanation := awx.StringField(job, "job_explanation"); explanation != "" {
			msg += " AWX said: " + explanation
		}
		msg += " Tick Ignore Job Failure to carry on regardless."
		return failure(out, msg), nil
	}

	summary := fmt.Sprintf("AWX %s %d (%s) finished with status %q after %ss.",
		kindLabel(kind), jobID, template, status, awx.IDString(job["elapsed"]))
	if awx.BoolInput("include_stdout", inputs) && kind == awx.JobKindWorkflowJob {
		// A sliced launch (job_slice_count > 1) produces a WORKFLOW job, whose stdout
		// is ALWAYS empty — it has no output of its own. Explain that, exactly as Wait
		// for Job does, rather than hand the operator a blank Output field and a
		// success message with no clue why.
		summary += " A workflow job has no output of its own — list its nodes and fetch the output of the child job you want."
	} else if res.StdoutTruncated {
		summary += " The output was truncated — raise Max Output Bytes, or use Get Job Output."
	}
	return success(out, summary), nil
}

// launchBody assembles the launch payload, mapping each input to the AWX body field
// it drives. The field NAMES matter: awx.ValidatePrompts gates exactly these keys
// against the template's ask_*_on_launch flags, so a typo here would turn the
// ignored-fields guard into a no-op for that field.
//
// Additional Fields is merged LAST, so a power user's key overrides a first-class
// input — and so anything smuggled in there still goes through the same pre-flight.
func launchBody(inputs []*core.Connection) (map[string]interface{}, error) {
	body := map[string]interface{}{}

	// Survey answers live here too, keyed by the survey variable name. A multiselect
	// answer is a JSON ARRAY (["osmp-01"]) where a multiplechoice is a scalar — the
	// survey validation in awx.ValidateSurvey enforces the difference before launch.
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
	awx.SetIfPresent(body, inputs, "job_tags", "job_tags")
	awx.SetIfPresent(body, inputs, "skip_tags", "skip_tags")
	awx.SetIfPresent(body, inputs, "job_type", "job_type")
	// verbosity rides on a String input (it is a dropdown) but AWX wants a JSON int.
	if err := awx.SetIntIfPresent(body, inputs, "verbosity", "verbosity"); err != nil {
		return nil, err
	}
	awx.SetIfPresent(body, inputs, "scm_branch", "scm_branch")
	awx.SetIntListIfPresent(body, inputs, "credentials", "credentials")

	passwords, err := awx.OptionalJSONObject("credential_passwords", inputs)
	if err != nil {
		return nil, err
	}
	if len(passwords) > 0 {
		body["credential_passwords"] = passwords
	}

	if err := awx.SetIntIfPresent(body, inputs, "execution_environment", "execution_environment_id"); err != nil {
		return nil, err
	}
	awx.SetIntListIfPresent(body, inputs, "labels", "labels")
	awx.SetIntListIfPresent(body, inputs, "instance_groups", "instance_groups")

	if err := awx.SetIntIfPresent(body, inputs, "forks", "forks"); err != nil {
		return nil, err
	}
	if err := awx.SetIntIfPresent(body, inputs, "job_slice_count", "job_slice_count"); err != nil {
		return nil, err
	}
	// The AWX-side job timeout. Named launch_timeout on the node so it cannot be
	// confused with — or shadowed by — the wait's own Timeout (seconds).
	if err := awx.SetIntIfPresent(body, inputs, "timeout", "launch_timeout"); err != nil {
		return nil, err
	}

	// diff_mode is sent ONLY when ticked, never as an explicit false.
	//
	// The editor cannot represent "untouched" once a checkbox has been touched: it
	// has no third state, and clearing it writes false, not null. An explicit
	// diff_mode=false would then be gated by ask_diff_mode_on_launch like any other
	// prompt field — so an operator who ticked the box and un-ticked it again would
	// be refused by the ignored-fields guard on every subsequent launch, with no way
	// to clear the field from the UI. A false here is a no-op anyway (AWX falls back
	// to the template's own value), and Additional Fields — {"diff_mode": false} —
	// remains the escape hatch for genuinely forcing it off on a template that
	// prompts for it.
	if awx.BoolInput("diff_mode", inputs) {
		body["diff_mode"] = true
	}

	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	return body, nil
}

// jobOutputs flattens a job record into this action's outputs, overriding the id
// and the UI link with the ones awx.LaunchedJob resolved.
//
// The override is what makes a SLICED launch safe: its 201 is a workflow-job record
// whose "id" we must not trust to be the thing we are about to poll.
func jobOutputs(auth awx.Auth, kind string, jobID int64, job map[string]interface{}) map[string]interface{} {
	out := awx.JobOutputs(auth, kind, job)
	out["job_id"] = awx.IDString(jobID)
	out["job_kind"] = kind
	out["job_url"] = awx.JobURL(auth, kind, jobID)
	return out
}

// templateName is the template's own name, taken from the pre-flight body, so every
// message names the thing the operator picked rather than a bare id.
func templateName(cfg awx.LaunchConfig, id int64) string {
	if data, ok := cfg.Raw["job_template_data"].(map[string]interface{}); ok {
		if name := awx.StringField(data, "name"); name != "" {
			return name
		}
	}
	return fmt.Sprintf("job template %d", id)
}

// jobEndedBadly reports whether a terminal job ended in a state the operator would
// call a failure.
//
// It checks the STATUS as well as the `failed` flag: a cancelled job is the case
// that matters — it is emphatically not a success, and reporting one as such would
// let a flow carry on as though the playbook had run. The contract stated on the
// Ignore Job Failure checkbox is failed/error/canceled, and this is it.
func jobEndedBadly(status string, failed bool) bool {
	switch status {
	case "failed", "error", "canceled":
		return true
	}
	return failed
}

// kindLabel names a job kind the way a human would.
func kindLabel(kind string) string {
	if kind == awx.JobKindWorkflowJob {
		// A sliced job template produces one of these, which surprises people.
		return "workflow job"
	}
	return "job"
}

func statusOrUnknown(status string) string {
	if status == "" {
		return "still running"
	}
	return status
}

func success(out map[string]interface{}, summary string) map[string]interface{} {
	out["tool_result"] = summary
	out["success"] = true
	out["error"] = ""
	return out
}

// failure marks the node failed while KEEPING every output already established —
// the job id above all. By the time most failures here are reachable the job is
// already running in AWX, and an operator who cannot see its number cannot go and
// look at it. Returned with a NIL error, so the flow keeps walking.
func failure(out map[string]interface{}, msg string) map[string]interface{} {
	out["tool_result"] = msg
	out["success"] = false
	out["error"] = msg
	return out
}
