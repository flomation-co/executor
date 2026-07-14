// Package awx_webhook triggers a flow when an AWX / AAP job starts, succeeds, fails
// or is canceled.
//
// The action itself does almost nothing, and that is deliberate. AWX has no "job
// finished" webhook in the usual sense: its outbound mechanism is a NOTIFICATION
// TEMPLATE of type "webhook", created in AWX and attached to the chosen job template
// by three per-event relations. Flomation creates, attaches and tears that down
// automatically, and every bit of it lives in the launch service — `grep -rn
// trigger_state executor/` returns nothing. By the time Execute runs, launch has
// already authenticated the delivery, confirmed it against AWX, filtered it to the
// events this flow asked for, and injected the job's fields as inputs. This file's
// only job is to echo those fields as outputs, strip the configuration, and
// synthesise a human summary.
//
// ★ STRIPPING THE CONFIG IS THE SECURITY BOUNDARY. Execute echoes every input it is
// handed, so `configInputs` is the whole of what stands between the AWX API token —
// and the password, and the rest of the auth block — and every downstream node in
// the flow. An input added to Inputs but forgotten in configInputs leaks it silently.
// Two tests pin that invariant in both directions: every Input must be in the
// denylist, and no declared Output may be (the latter would make Execute drop an
// injected event field on the floor).
//
// Three AWX facts shape the inputs:
//
//   - ★ AWX HAS ONLY THREE EVENTS, AND "error" IS A CATCH-ALL. Internally AWX maps
//     succeeded→success, running→started and everything else→error, so a FAILED, an
//     ERRORED and a CANCELED job all fire the same `notification_templates_error`
//     relation. (It is `_error`, not `_failure` — the "_failure" spelling is an
//     internal Django related_name that never appears in a URL, and is the classic
//     way to get a silent 404 here.) The node therefore offers FOUR logical events
//     and discriminates on the payload's `status` field in launch, so a flow that
//     asked only for Job Canceled is not woken by every failure in the estate.
//
//   - ★ THE NOTIFICATION IS UNSIGNED. There is no HMAC, no signature header, no
//     timestamp and no replay protection anywhere in AWX's outbound path. Flomation
//     mints a per-trigger secret and has AWX present it as HTTP Basic auth, because
//     `password` is the ONLY field AWX encrypts at rest — the `headers` map reads
//     back in PLAINTEXT to any AWX user who can view the template, so a shared secret
//     must never be put there. On top of that, launch calls back to AWX to confirm
//     the job really is in the state the pushed payload claims. Skip AWX Verification
//     turns that callback off.
//
//   - ★ DELIVERY IS AT-MOST-ONCE. A 4xx, 5xx, timeout or connection error marks the
//     notification failed and AWX NEVER RETRIES it (its MAX_RETRIES budget only
//     follows redirects). An event raised while Flomation is unreachable is lost for
//     good. This trigger must not be the sole record of a job having run.
package awx_webhook

import (
	"encoding/json"
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX Job Trigger"
	Description  = "Triggers a flow when an AWX / AAP job starts, succeeds, fails or is canceled. Flomation registers the notification in AWX automatically. Note that AWX sends each notification once and never retries, so an event raised while Flomation is unreachable is lost."
	Website      = "https://www.flomation.co"
	// The bare base, no badge — the trigger convention (cf. woocommerce, monday).
	Icon = "ansible"
	Date = "14/07/2026"
	Type = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	// ---- AUTH (re-declared verbatim from awx.AuthInputs; the manifest AST-parser
	// cannot see through a package-level variable, so the literals are mandatory).
	// Launch uses these to create the notification template in AWX, and again on each
	// delivery to confirm the job's status. ----
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

	// ---- WHAT TO WATCH ----
	// The two pickers are mutually exclusive and both Required. That is safe: the
	// editor skips the required-field check on an input its visible_when has hidden.
	{Name: "template_kind", Type: core.ConnectionTypeString, Label: "Template Type", Placeholder: "A job template, or a workflow template", Options: []core.ConnectionOption{
		{Name: "Job Template", Value: "job_template"},
		{Name: "Workflow Template", Value: "workflow_job_template"},
	}},
	// Live dropdowns (awx-job-templates / awx-workflow-templates) — the options are
	// fetched from the operator's own AWX by the api service; nothing is declared here.
	{Name: "job_template_id", Type: core.ConnectionTypeString, Label: "Job Template", Placeholder: "Pick the job template to watch", Required: true, Visible: &core.VisibleWhen{Field: "template_kind", Values: []string{"", "job_template"}}},
	{Name: "workflow_template_id", Type: core.ConnectionTypeString, Label: "Workflow Template", Placeholder: "Pick the workflow template to watch", Required: true, Visible: &core.VisibleWhen{Field: "template_kind", Values: []string{"workflow_job_template"}}},

	// AWX itself has only three events and treats "error" as a catch-all, so Failed
	// and Canceled both attach to the same AWX relation; launch tells them apart by
	// the job's status and drops any delivery this flow did not ask for.
	{Name: "events", Type: core.ConnectionTypeMultiSelect, Label: "Events", Required: true, Options: []core.ConnectionOption{
		{Name: "Job Started", Value: "started"},
		{Name: "Job Succeeded", Value: "successful"},
		{Name: "Job Failed or Errored", Value: "failed"},
		{Name: "Job Canceled", Value: "canceled"},
	}},

	// Phrased as an opt-OUT because the manifest does not harvest Value: a
	// "verify_with_awx, default true" checkbox would render UNTICKED in the editor and
	// silently mean false. Unticked here is the safe behaviour — Flomation verifies.
	{Name: "skip_awx_verification", Type: core.ConnectionTypeBoolean, Label: "Skip AWX Verification", Placeholder: "Leave unticked. AWX does not sign its webhooks, so Flomation confirms each one against AWX before firing the flow. Tick only if these credentials cannot read jobs."},
}

// Outputs are the fields of AWX's default notification body — `notification_data()`,
// which the webhook backend sends verbatim as `{{ job_metadata }}` — plus the few
// launch derives.
//
// `event` (our logical event), `failed` and `triggered_at` are DERIVED: AWX's payload
// carries neither a `failed` flag nor an event name, only `status`. `job_url` is
// AWX's top-level `url`, which is the UI link (https://awx/#/jobs/playbook/42) and
// not the API path. `extra_vars` is a JSON *string* on the wire, not an object —
// AWX renders it through display_extra_vars(), which redacts survey passwords.
var Outputs = [...]core.Connection{
	{Name: "content", Type: core.ConnectionTypeString, Label: "Event Summary"},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event"},
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID"},
	{Name: "job_name", Type: core.ConnectionTypeString, Label: "Job Name"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "Job URL"},
	{Name: "created_by", Type: core.ConnectionTypeString, Label: "Launched By"},
	{Name: "started", Type: core.ConnectionTypeString, Label: "Started"},
	{Name: "finished", Type: core.ConnectionTypeString, Label: "Finished"},
	{Name: "traceback", Type: core.ConnectionTypeString, Label: "Traceback"},
	{Name: "inventory", Type: core.ConnectionTypeString, Label: "Inventory"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project"},
	{Name: "playbook", Type: core.ConnectionTypeString, Label: "Playbook"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit"},
	{Name: "extra_vars", Type: core.ConnectionTypeString, Label: "Extra Variables (JSON)"},
	{Name: "hosts", Type: core.ConnectionTypeObject, Label: "Host Status Counts"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Raw JSON Body"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// configInputs are the trigger's own configuration fields, which must NEVER be echoed
// as outputs: they carry the AWX credentials and the registration settings, and
// Execute copies every input it is not told to drop.
//
// ★ Every name in Inputs must appear here, plus __node_id — the internal marker
// launch attaches so the executor injects the event into the right trigger node in a
// multi-trigger flow. TestEveryConfigInputIsInTheDenylist enforces exactly that, and
// TestNoDeclaredOutputIsStrippedByTheDenylist enforces the converse (a config name
// that collided with an output name would silently blank that output — the trap
// Monday.com's board_id sits in). No AWX config name collides with an output name:
// the pickers are job_template_id / workflow_template_id, never job_id.
var configInputs = map[string]bool{
	"awx_url":               true,
	"auth_method":           true,
	"api_token":             true,
	"awx_username":          true,
	"awx_password":          true,
	"allow_insecure":        true,
	"api_prefix":            true,
	"template_kind":         true,
	"job_template_id":       true,
	"workflow_template_id":  true,
	"events":                true,
	"skip_awx_verification": true,
	"__node_id":             true,
}

// Execute runs at flow-execution time with the AWX notification payload already
// parsed, verified and injected by launch. It echoes the event fields, drops the
// configuration, and derives the summary line.
//
// Trigger actions are exempt from the success / error / tool_result contract.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing AWX webhook trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil && !configInputs[input.Name] {
			result[input.Name] = input.Value
		}
	}

	result["content"] = buildContentSummary(result)

	return result, nil
}

// str stringifies an injected payload value.
//
// ★ It MUST tolerate a JSON number. AWX sends the job id as a NUMBER ("id": 42), so
// if it reaches us unconverted it arrives as a float64, and a plain v.(string)
// assertion would yield "" — silently dropping the job id out of every summary. The
// integer widths and json.Number are handled for the same reason.
func str(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case int:
		return strconv.Itoa(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

// buildContentSummary is the one-line human summary shown in the flow's run history,
// e.g. `[AWX] Deploy Web #42 — successful`. It degrades cleanly: the started event
// carries no finished time and a workflow job carries no playbook, so nothing here
// may assume any particular field is present.
func buildContentSummary(data map[string]interface{}) string {
	name := str(data["job_name"])
	id := str(data["job_id"])

	// status is what AWX actually sent; event is our logical name for it, and is the
	// better fallback than nothing when a payload arrives without a status.
	status := str(data["status"])
	if status == "" {
		status = str(data["event"])
	}

	var subject string
	switch {
	case name != "" && id != "":
		subject = fmt.Sprintf("%s #%s", name, id)
	case name != "":
		subject = name
	case id != "":
		subject = fmt.Sprintf("job #%s", id)
	}

	switch {
	case subject != "" && status != "":
		return fmt.Sprintf("[AWX] %s — %s", subject, status)
	case subject != "":
		return fmt.Sprintf("[AWX] %s", subject)
	case status != "":
		return fmt.Sprintf("[AWX] Job %s", status)
	default:
		return "[AWX] Job event"
	}
}
