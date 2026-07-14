// Package infrastructure_awx_inventory_source_sync re-imports the hosts of one
// inventory source.
//
// Two AWX quirks shape this action:
//
//   - can_update. POST {root}inventory_sources/{id}/update/ answers 405 Method
//     Not Allowed — NOT 400 — when the source cannot be synced (a manual/file
//     source, or a source-control source with no Source Project). A bare "HTTP
//     405" is meaningless to an operator, so we GET the same URL first, read
//     {"can_update": bool}, and explain the actual reason.
//
//   - the 202. A successful sync answers 202 ACCEPTED and puts the new job's id
//     at BOTH .inventory_update and .id, with type "inventory_update". That
//     polymorphism is absorbed by awx.LaunchedJob, and the resulting job is
//     polled as JobKindInventoryUpdate — polling /jobs/ for it would 404.
package infrastructure_awx_inventory_source_sync

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Sync Inventory Source"
	Description  = "Re-import hosts from an inventory's external source (a cloud provider, a git repo, etc)."
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Choose the inventory the source belongs to — this narrows the Inventory Source list below"},
	{Name: "inventory_source_id", Type: core.ConnectionTypeString, Label: "Inventory Source", Placeholder: "The source to re-import hosts from", Required: true},
	{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Label: "Wait for Completion", Placeholder: "Hold the flow until AWX has finished importing, then return the result"},
	{Name: "poll_interval_seconds", Type: core.ConnectionTypeInteger, Label: "Poll Interval (seconds)", Placeholder: "How often to check the sync — default 3s", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "Stop waiting after this long — default 300s (max 3600)", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "ignore_job_failure", Type: core.ConnectionTypeBoolean, Label: "Ignore Sync Failure", Placeholder: "By default this node fails when the AWX sync ends failed/error/canceled. Tick to carry on regardless.", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
	{Name: "include_stdout", Type: core.ConnectionTypeBoolean, Label: "Include Output", Placeholder: "Return the import log — useful when a sync fails and you need to know why", Visible: &core.VisibleWhen{Field: "wait_for_completion", Values: []string{"true"}}},
}

var Outputs = [...]core.Connection{
	{Name: "inventory_update_id", Type: core.ConnectionTypeString, Label: "Inventory Sync ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "timed_out", Type: core.ConnectionTypeBoolean, Label: "Timed Out"},
	{Name: "stdout", Type: core.ConnectionTypeString, Label: "Output"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "AWX Link"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Inventory Sync"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	sourceID, err := awx.RequiredInt("inventory_source_id", "Inventory Source", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	updatePath := fmt.Sprintf("inventory_sources/%d/update/", sourceID)

	// ★ Pre-check can_update. Without this the operator gets a naked 405.
	probe, err := awx.GetResource(ctx, auth, updatePath, nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if !awx.BoolField(probe, "can_update") {
		return awx.ErrorResult(fmt.Sprintf(
			"AWX will not sync inventory source %d: it is not syncable as configured. A manual (file) source has nothing to import, and a source-control source needs a Source Project set. Open the source in AWX ▸ Inventories ▸ your inventory ▸ Sources and check its source type and credential.",
			sourceID)), nil
	}

	resp, err := awx.Do(ctx, auth, http.MethodPost, updatePath, nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if resp.StatusCode == http.StatusMethodNotAllowed {
		// The pre-check said yes and AWX still refused — the source changed under
		// us, or AWX disagrees. Either way, never show the raw 405.
		return awx.ErrorResult(fmt.Sprintf(
			"AWX refused to sync inventory source %d — it is not syncable as configured (a manual source has nothing to import; a source-control source needs a Source Project). Check the source in AWX ▸ Inventories ▸ Sources.",
			sourceID)), nil
	}
	if err := awx.CheckResponse(auth, resp, http.StatusAccepted); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	launched, err := awx.DecodeObject(resp)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	updateID, kind, err := awx.LaunchedJob(launched, awx.JobKindInventoryUpdate)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	if !awx.BoolInput("wait_for_completion", inputs) {
		out := awx.JobOutputs(auth, kind, launched)
		return finish(out, updateID, false,
			fmt.Sprintf("Started inventory sync %d for source %d (status %s). It is running in AWX — tick “Wait for the sync to finish” to hold the flow until it completes.",
				updateID, sourceID, awx.StringField(launched, "status"))), nil
	}

	pollSeconds, _ := awx.OptionalInt("poll_interval_seconds", inputs)
	if pollSeconds <= 0 {
		// The manifest cannot carry a default Value, so it lives here and in the
		// placeholder. An inventory sync is short, so it is polled faster than a job.
		pollSeconds = 3
	}
	timeoutSeconds, _ := awx.OptionalInt("timeout_seconds", inputs)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	includeStdout := awx.BoolInput("include_stdout", inputs)

	res, err := awx.WaitForJob(ctx, auth, kind, updateID, awx.WaitOpts{
		PollIntervalSeconds: pollSeconds,
		TimeoutSeconds:      timeoutSeconds,
		IncludeStdout:       includeStdout,
		WaitForEvents:       includeStdout,
	})
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	out := awx.JobOutputs(auth, kind, res.Job)
	out["stdout"] = res.Stdout

	if res.TimedOut {
		return finish(out, updateID, true, fmt.Sprintf(
			"Inventory sync %d was still running after %ds, so this node stopped waiting. The sync is STILL RUNNING in AWX — open %s to follow it.",
			updateID, awx.ClampWaitSeconds(timeoutSeconds), awx.JobURL(auth, kind, updateID))), nil
	}

	status := awx.StringField(res.Job, "status")
	if awx.BoolField(res.Job, "failed") && !awx.BoolInput("ignore_job_failure", inputs) {
		msg := fmt.Sprintf("Inventory sync %d for source %d ended %s. Nothing was imported. Open %s for the import log — tick “Succeed even if the sync fails” to carry on regardless.",
			updateID, sourceID, status, awx.JobURL(auth, kind, updateID))
		if explanation := awx.StringField(res.Job, "job_explanation"); explanation != "" {
			msg += " AWX said: " + explanation
		}
		out["tool_result"] = msg
		out["success"] = false
		out["error"] = msg
		out["inventory_update_id"] = awx.IDString(updateID)
		out["timed_out"] = false
		return out, nil
	}

	return finish(out, updateID, false,
		fmt.Sprintf("Inventory sync %d for source %d finished: %s.", updateID, sourceID, status)), nil
}

// finish stamps the standard success envelope onto a JobOutputs map and renames
// job_id to the inventory-update id this action promises.
func finish(out map[string]interface{}, updateID int64, timedOut bool, summary string) map[string]interface{} {
	if _, ok := out["stdout"]; !ok {
		out["stdout"] = ""
	}
	out["inventory_update_id"] = awx.IDString(updateID)
	out["timed_out"] = timedOut
	out["tool_result"] = summary
	out["success"] = !timedOut
	if timedOut {
		out["error"] = summary
	} else {
		out["error"] = ""
	}
	return out
}
