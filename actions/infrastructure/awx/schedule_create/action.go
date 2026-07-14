// Package infrastructure_awx_schedule_create creates an AWX schedule — a
// recurring, controller-side launch of a job template.
//
// Two things make this action more than a POST:
//
//   - THE RECURRENCE RULE. AWX schedules are iCal RRULEs, which no non-technical
//     operator can be expected to write from memory. So the action calls AWX's own
//     POST schedules/preview/ FIRST, which validates the rule and returns the next
//     ten run times — and those go straight into the `preview` output, so the
//     operator can SEE when their schedule will fire before anything is created.
//     A bad rule is reported in AWX's own words, before a schedule exists.
//
//   - PROMPT FIELDS ARE REJECTED HERE, NOT IGNORED. A manual launch silently drops
//     an override the template does not prompt for; a schedule is a hard 400
//     ({"limit": "Field is not configured to prompt on launch."}). So the action
//     runs exactly the same pre-flight as Launch Job Template (awx.ValidateLaunch)
//     and soft-fails naming the field and the checkbox to tick in AWX.
package infrastructure_awx_schedule_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	awx "flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "AWX: Create Schedule"
	Description  = "Schedule a job template to run automatically — hourly, daily, weekly, or on a custom recurrence rule."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+plus"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// ---- AUTH BLOCK (identical on every AWX action; see awx.AuthInputs) ----
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

	// ---- THE SCHEDULE ----
	{Name: "job_template_id", Type: core.ConnectionTypeString, Label: "Job Template", Placeholder: "The job template to run on this schedule", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Schedule Name", Placeholder: "Nightly patching — must be unique for this job template", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this schedule is for"},
	{
		Name:        "rrule",
		Type:        core.ConnectionTypeString,
		Label:       "Recurrence Rule",
		Placeholder: `DTSTART;TZID=Europe/London:20260801T090000 RRULE:FREQ=DAILY;INTERVAL=1 — every day at 09:00 London time. Change FREQ to HOURLY / WEEKLY / MONTHLY, and add BYDAY=MO,WE,FR for particular days.`,
		Required:    true,
	},
	{Name: "enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled", Placeholder: "Leave untouched to let AWX enable it (its default). Untick to create the schedule paused."},

	// ---- PROMPT OVERRIDES — only accepted if the template's matching
	// 'Prompt on launch' is on; AWX hard-rejects them otherwise, so the node
	// pre-flights and refuses first. ----
	{Name: "extra_data", Type: core.ConnectionTypeObject, Label: "Extra Variables / Survey Answers", Placeholder: `{"target_env":"prod"} — survey answers go here too, keyed by the survey variable name`},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "web*:&prod — Ansible host pattern"},
	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Run against this inventory instead of the template's own"},
	{Name: "job_tags", Type: core.ConnectionTypeString, Label: "Job Tags", Placeholder: "install,configure — only run tasks with these tags"},
	{Name: "skip_tags", Type: core.ConnectionTypeString, Label: "Skip Tags", Placeholder: "reboot — skip tasks with these tags"},
	{Name: "verbosity", Type: core.ConnectionTypeString, Label: "Verbosity", Options: []core.ConnectionOption{
		{Name: "0 (Normal)", Value: "0"},
		{Name: "1 (Verbose)", Value: "1"},
		{Name: "2 (More Verbose)", Value: "2"},
		{Name: "3 (Debug)", Value: "3"},
		{Name: "4 (Connection Debug)", Value: "4"},
		{Name: "5 (WinRM Debug)", Value: "5"},
	}},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"execution_environment":2} — any other AWX schedule field; overrides the fields above`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Schedule ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Schedule"},
	{Name: "next_run", Type: core.ConnectionTypeString, Label: "Next Run"},
	{Name: "timezone", Type: core.ConnectionTypeString, Label: "Time Zone"},
	{Name: "preview", Type: core.ConnectionTypeObject, Label: "Next Run Times"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err // the ONLY hard failure: the node is mis-configured
	}

	ctx, cancel := awx.Context()
	defer cancel()

	templateID, err := awx.RequiredInt("job_template_id", "Job Template", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	name, err := awx.RequiredString("name", "Schedule Name", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	rrule, err := awx.RequiredString("rrule", "Recurrence Rule", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	extraData, err := awx.OptionalJSONObject("extra_data", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// ---- PRE-FLIGHT ---------------------------------------------------------
	// The schedule body names its prompt fields slightly differently from a launch
	// body (extra_data, not extra_vars; inventory, not inventory_id), so the
	// pre-flight is handed a launch-shaped copy. ValidateLaunch then does exactly
	// what Launch Job Template does: refuse any override this template is not
	// configured to prompt for, and validate the survey answers client-side.
	//
	// allowIgnored is false and there is no escape hatch on this action: unlike a
	// launch, AWX does not silently ignore these fields — it 400s — so "send it
	// anyway" would never succeed.
	launchShaped := map[string]interface{}{}
	if len(extraData) > 0 {
		launchShaped["extra_vars"] = extraData
	}
	awx.SetIfPresent(launchShaped, inputs, "limit", "limit")
	awx.SetIfPresent(launchShaped, inputs, "job_tags", "job_tags")
	awx.SetIfPresent(launchShaped, inputs, "skip_tags", "skip_tags")
	if err := awx.SetIntIfPresent(launchShaped, inputs, "inventory", "inventory_id"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.SetIntIfPresent(launchShaped, inputs, "verbosity", "verbosity"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if _, err := awx.ValidateLaunch(ctx, auth, awx.TemplateKindJob, templateID, launchShaped, false); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// ---- VALIDATE THE RECURRENCE RULE, AND SHOW THE OPERATOR WHEN IT FIRES ----
	// AWX validates the rule for us and hands back the next ten occurrences. Doing
	// this before the create means a malformed rule never leaves a half-made
	// schedule behind, and the operator gets AWX's own wording for what is wrong.
	preview, err := awx.CreateResource(ctx, auth, "schedules/preview/", map[string]interface{}{"rrule": rrule})
	if err != nil {
		return awx.ErrorResult(fmt.Sprintf(
			"AWX would not accept that recurrence rule: %s "+
				"A rule needs a time-zone-aware DTSTART and an INTERVAL, for example "+
				"DTSTART;TZID=Europe/London:20260801T090000 RRULE:FREQ=DAILY;INTERVAL=1. "+
				"(FREQ=SECONDLY is not allowed, COUNT and UNTIL cannot both appear, and there must be exactly one DTSTART and one RRULE.)",
			err.Error())), nil
	}

	// ---- CREATE -------------------------------------------------------------
	body := map[string]interface{}{
		"unified_job_template": templateID,
		"name":                 name,
		"rrule":                rrule,
	}
	awx.SetIfPresent(body, inputs, "description", "description")
	// Tri-state: AWX defaults a schedule to enabled, and the manifest cannot carry
	// a default, so an untouched checkbox must be OMITTED rather than sent as false.
	awx.SetBoolIfSet(body, inputs, "enabled", "enabled")
	if len(extraData) > 0 {
		body["extra_data"] = extraData
	}
	awx.SetIfPresent(body, inputs, "limit", "limit")
	awx.SetIfPresent(body, inputs, "job_tags", "job_tags")
	awx.SetIfPresent(body, inputs, "skip_tags", "skip_tags")
	if err := awx.SetIntIfPresent(body, inputs, "inventory", "inventory_id"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.SetIntIfPresent(body, inputs, "verbosity", "verbosity"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	schedule, err := awx.CreateResource(ctx, auth, "schedules/", body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	nextRun := awx.StringField(schedule, "next_run")
	summary := fmt.Sprintf("Created schedule %q (ID %s)", name, awx.IDString(schedule["id"]))
	if nextRun != "" {
		summary += ", next run " + nextRun
	}

	out := awx.ObjectResult(schedule, summary)
	out["next_run"] = nextRun
	out["timezone"] = awx.StringField(schedule, "timezone")
	out["preview"] = preview
	return out, nil
}
