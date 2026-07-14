// Package infrastructure_awx_job_stdout_get fetches the plain-text playbook
// output of an AWX job, ad-hoc command or sync.
package infrastructure_awx_job_stdout_get

import (
	"context"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Job Output"
	Description  = "Fetch the plain-text playbook output (stdout) of an AWX job, ad-hoc command or sync."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+file-lines"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

const (
	// eventSettleTimeout bounds the extra wait for AWX to finish writing a job's
	// events. AWX writes them to Postgres asynchronously: status flips to
	// successful the instant the runner exits, but the output may still be
	// flushing, so reading it immediately yields a TRUNCATED log.
	eventSettleTimeout  = 30 * time.Second
	eventSettleInterval = 2 * time.Second
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
	// No Workflow Job here: a workflow has no output of its own — list its nodes
	// and fetch the output of the child job you want.
	{Name: "job_kind", Type: core.ConnectionTypeString, Label: "Job Type", Placeholder: "Which kind of AWX job — a playbook Job unless you know otherwise", Options: []core.ConnectionOption{
		{Name: "Job", Value: "job"},
		{Name: "Ad-Hoc Command", Value: "ad_hoc_command"},
		{Name: "Project Sync", Value: "project_update"},
		{Name: "Inventory Sync", Value: "inventory_update"},
	}},
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Placeholder: "42 — the AWX job ID, e.g. from a Launch or Wait node", Required: true},
	{Name: "max_bytes", Type: core.ConnectionTypeInteger, Label: "Max Bytes", Placeholder: "Truncate the output at this many bytes — default 1 MB (1048576)"},
	{Name: "wait_for_events", Type: core.ConnectionTypeBoolean, Label: "Wait for Events", Placeholder: "Wait for AWX to finish writing the job events before reading (recommended — otherwise the output can be truncated)"},
}

var Outputs = [...]core.Connection{
	{Name: "stdout", Type: core.ConnectionTypeString, Label: "Output"},
	{Name: "byte_count", Type: core.ConnectionTypeInteger, Label: "Byte Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
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
	if kind == "" {
		kind = awx.JobKindJob
	}
	if kind == awx.JobKindWorkflowJob {
		return awx.ErrorResult("A workflow job has no output of its own — its playbook output lives on the child jobs. Use List Workflow Job Nodes to find the child job, then Get Job Output on that."), nil
	}
	path, err := awx.JobKindPath(kind)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	id, err := awx.RequiredInt("job_id", "Job ID", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	maxBytes, set := awx.OptionalInt("max_bytes", inputs)
	if !set || maxBytes <= 0 {
		maxBytes = awx.DefaultStdoutMaxBytes // the manifest cannot carry a default Value
	}

	// AWX writes job events asynchronously — the log can still be flushing after
	// the job has gone terminal. settled=false only means we read early; it is
	// reported, never treated as a failure.
	settled := true
	if awx.BoolInput("wait_for_events", inputs) {
		settled = settleEvents(ctx, auth, path, id)
	}

	// FetchStdout always uses ?format=txt_download and string-guards AWX's
	// "Standard Output too large to display" sentence, which it serves with a
	// 200 — a naive client stores that apology AS the playbook output.
	text, truncated, err := awx.FetchStdout(ctx, auth, kind, id, maxBytes)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if !settled {
		truncated = true
	}

	summary := fmt.Sprintf("Fetched %d byte(s) of output from %s %d", len(text), label(kind), id)
	if truncated {
		summary += " — the output is incomplete"
		if !settled {
			summary += " (AWX had not finished writing this job's events)"
		} else {
			summary += fmt.Sprintf(" (truncated at the %d-byte Max Bytes cap)", maxBytes)
		}
	}

	return map[string]interface{}{
		"stdout":      text,
		"byte_count":  len(text),
		"truncated":   truncated,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}, nil
}

// settleEvents waits, briefly and boundedly, for AWX to report that it has
// finished writing this job's events.
//
// It returns immediately — reporting "not settled" rather than hanging — when the
// job is still running (its events will never settle while it runs) or when the
// record carries no event_processing_finished field at all.
func settleEvents(ctx context.Context, auth awx.Auth, path string, id int64) bool {
	deadline := time.Now().Add(eventSettleTimeout)
	for {
		job, err := awx.GetResource(ctx, auth, fmt.Sprintf("%s%d/", path, id), nil)
		if err != nil {
			return false
		}
		if _, present := job["event_processing_finished"]; !present {
			return false
		}
		if awx.BoolField(job, "event_processing_finished") {
			return true
		}
		if v, ok := job["finished"]; !ok || v == nil {
			return false // still running — read what output there is so far
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return false
		}

		timer := time.NewTimer(eventSettleInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
}

func label(kind string) string {
	switch kind {
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
