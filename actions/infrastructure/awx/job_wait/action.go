// Package infrastructure_awx_job_wait holds a flow until an AWX job finishes, then
// returns its final status, output and artifacts.
//
// Launching a job template already offers Wait for Completion as a flag, so this
// action exists for the case that flag cannot cover: waiting on a job somebody ELSE
// started — one that arrived on the AWX trigger, one a colleague kicked off from the
// AWX UI, one an earlier flow launched asynchronously. It takes a job ID and waits.
//
// It covers all five of AWX's "unified job" kinds (job, workflow job, ad-hoc
// command, project sync, inventory sync): their records and their status semantics
// are identical, so one Wait action serves them all.
//
// Two AWX behaviours shape the wait, both handled in awx.WaitForJob:
//
//   - The hot loop polls the LIST endpoint (?id=N), never the detail. AWX's
//     JobDetailSerializer runs two COUNT(*) queries over the job-events table on
//     EVERY detail GET, so polling the detail once a second is genuinely expensive
//     for the AWX database on a big job. One detail GET is taken at the end, for the
//     fields the list view strips (result_traceback, event_processing_finished,
//     host_status_counts).
//
//   - AWX writes job events ASYNCHRONOUSLY. A job's status flips to successful the
//     instant the runner exits, but its stdout and artifacts may still be flushing
//     to Postgres — so reading them the moment the status goes terminal yields
//     truncated or EMPTY results. The wait gates on event_processing_finished before
//     reading either.
//
// On a timeout the job is NOT cancelled unless Cancel on Timeout is ticked: silently
// killing a production job because a flow got bored is surprising and destructive.
// The node soft-fails with timed_out=true and the job's id and URL, so the operator
// can go and find it.
package infrastructure_awx_job_wait

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Wait for Job"
	Description  = "Hold the flow until an AWX job finishes, then return its final status, output and artifacts. Use this to wait on a job that was launched elsewhere — for example one that arrived on the AWX trigger."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+clock"
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
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Placeholder: "42 — the AWX job ID, e.g. from a Launch node or the AWX trigger", Required: true},
	{Name: "poll_interval_seconds", Type: core.ConnectionTypeInteger, Label: "Poll Interval (seconds)", Placeholder: "How often to check the job — default 5s"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "Give up waiting after this long — default 600s (max 3600)"},
	{Name: "cancel_on_timeout", Type: core.ConnectionTypeBoolean, Label: "Cancel on Timeout", Placeholder: "Cancel the job in AWX if the wait runs out. Leave unticked to let it carry on running."},
	{Name: "ignore_job_failure", Type: core.ConnectionTypeBoolean, Label: "Ignore Job Failure", Placeholder: "By default the node fails when the AWX job ends failed/error/canceled. Tick to succeed regardless."},
	{Name: "include_stdout", Type: core.ConnectionTypeBoolean, Label: "Include Output", Placeholder: "Also return the playbook's text output (stdout)"},
	{Name: "stdout_max_bytes", Type: core.ConnectionTypeInteger, Label: "Max Output Bytes", Placeholder: "Truncate the output at this many bytes — default 1048576 (1 MB)", Visible: &core.VisibleWhen{Field: "include_stdout", Values: []string{"true"}}},
}

var Outputs = [...]core.Connection{
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID"},
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Type"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "Job URL"},
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

	kind := awx.OptionalString("job_kind", inputs)
	if _, err := awx.JobKindPath(kind); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if kind == "" {
		kind = awx.JobKindJob
	}

	jobID, err := awx.RequiredInt("job_id", "Job ID", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	timeout, _ := awx.OptionalInt("timeout_seconds", inputs)
	poll, _ := awx.OptionalInt("poll_interval_seconds", inputs)
	stdoutMax, _ := awx.OptionalInt("stdout_max_bytes", inputs)
	includeStdout := awx.BoolInput("include_stdout", inputs)

	res, err := awx.WaitForJob(ctx, auth, kind, jobID, awx.WaitOpts{
		PollIntervalSeconds: poll,
		TimeoutSeconds:      timeout,
		CancelOnTimeout:     awx.BoolInput("cancel_on_timeout", inputs),
		IncludeStdout:       includeStdout,
		StdoutMaxBytes:      stdoutMax,
		// Artifacts and Host Status Counts are outputs of this node, and AWX writes
		// job events ASYNCHRONOUSLY — status flips to successful the instant the
		// runner exits, while the events are still flushing to Postgres. Without
		// this, a set_stats artifact read by the next node in the flow is
		// intermittently empty.
		WaitForEvents: true,
	})
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	job := res.Job
	url := awx.JobURL(auth, kind, jobID)

	out := awx.JobOutputs(auth, kind, job)
	out["job_id"] = awx.IDString(jobID)
	out["job_kind"] = kind
	out["job_url"] = url
	out["timed_out"] = res.TimedOut
	out["stdout"] = res.Stdout

	// A workflow job is a pure orchestration record: it has no events, no stdout and
	// no artifacts of its own — everything real lives on its child jobs. Emit nulls
	// rather than fabricating empty objects, so a downstream node can tell "nothing
	// here" from "nothing happened".
	if kind == awx.JobKindWorkflowJob {
		out["artifacts"] = nil
		out["host_status_counts"] = nil
		out["event_processing_finished"] = nil
	}

	status := awx.StringField(job, "status")

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
		msg := fmt.Sprintf("AWX %s %d finished with status %q. Open %s to see what failed.",
			kindLabel(kind), jobID, status, url)
		// Only an ERRORED job has a traceback: AWX could not run the playbook at all.
		// A FAILED one ran fine and the hosts failed — a different problem.
		if trace := awx.StringField(job, "result_traceback"); trace != "" {
			msg += " AWX could not run the playbook — see the Traceback output."
		}
		if explanation := awx.StringField(job, "job_explanation"); explanation != "" {
			msg += " AWX said: " + explanation
		}
		msg += " Tick Ignore Job Failure to carry on regardless."
		return failure(out, msg), nil
	}

	summary := fmt.Sprintf("AWX %s %d finished with status %q after %ss.",
		kindLabel(kind), jobID, status, awx.IDString(job["elapsed"]))
	if includeStdout && kind == awx.JobKindWorkflowJob {
		summary += " A workflow job has no output of its own — list its nodes and fetch the output of the child job you want."
	} else if res.StdoutTruncated {
		summary += " The output was truncated — raise Max Output Bytes, or use Get Job Output."
	}
	return success(out, summary), nil
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
	switch kind {
	case awx.JobKindWorkflowJob:
		return "workflow job"
	case awx.JobKindAdHocCommand:
		return "ad-hoc command"
	case awx.JobKindProjectUpdate:
		return "project sync"
	case awx.JobKindInventoryUpdate:
		return "inventory sync"
	default:
		return "job"
	}
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

// failure marks the node failed while KEEPING every output already established — the
// job id and URL above all, since on a timeout the job is still running in AWX and
// an operator who cannot see its number cannot go and look at it. Returned with a
// NIL error, so the flow keeps walking.
func failure(out map[string]interface{}, msg string) map[string]interface{} {
	out["tool_result"] = msg
	out["success"] = false
	out["error"] = msg
	return out
}
