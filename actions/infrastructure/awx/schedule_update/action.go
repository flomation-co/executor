// Package infrastructure_awx_schedule_update changes an AWX schedule.
//
// The overwhelmingly common use is pause/resume, so Enabled is the first field
// after the picker. It is a three-way dropdown — Leave unchanged / Enabled / Paused
// — NOT a checkbox: a plain checkbox renders unticked and the manifest cannot carry
// a default, so a non-technical operator following "untick to pause" could never
// actually pause a schedule (the box is already unticked, and the only gestures that
// send enabled=false are unreachable to them). The dropdown maps through
// SetBoolIfSet, so "Leave unchanged" (the empty value) is still OMITTED from the
// PATCH rather than sent as false.
package infrastructure_awx_schedule_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	awx "flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "AWX: Update Schedule"
	Description  = "Change a schedule — most often to pause or resume it."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+pencil"
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

	// ---- THE CHANGE ----
	{Name: "schedule_id", Type: core.ConnectionTypeString, Label: "Schedule", Placeholder: "The schedule to change", Required: true},
	{Name: "enabled", Type: core.ConnectionTypeString, Label: "Enabled", Placeholder: "Pause or resume the schedule — leave unchanged to keep its current state", Options: []core.ConnectionOption{
		{Name: "Leave unchanged", Value: ""},
		{Name: "Enabled", Value: "true"},
		{Name: "Paused", Value: "false"},
	}},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Schedule Name", Placeholder: "Leave blank to keep the current name"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Leave blank to keep the current description"},
	{Name: "rrule", Type: core.ConnectionTypeString, Label: "Recurrence Rule", Placeholder: `DTSTART;TZID=Europe/London:20260801T090000 RRULE:FREQ=DAILY;INTERVAL=1 — leave blank to keep the current rule`},
	{Name: "extra_data", Type: core.ConnectionTypeObject, Label: "Extra Variables / Survey Answers", Placeholder: `{"target_env":"prod"} — replaces the schedule's current variables`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"limit":"web*"} — any other AWX schedule field; overrides the fields above`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Schedule ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Schedule"},
	{Name: "enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled"},
	{Name: "next_run", Type: core.ConnectionTypeString, Label: "Next Run"},
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

	scheduleID, err := awx.RequiredInt("schedule_id", "Schedule", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	// Three-way: "Leave unchanged" (empty) is OMITTED; Enabled/Paused send true/false.
	awx.SetBoolIfSet(body, inputs, "enabled", "enabled")
	awx.SetIfPresent(body, inputs, "name", "name")
	awx.SetIfPresent(body, inputs, "description", "description")
	awx.SetIfPresent(body, inputs, "rrule", "rrule")
	if err := awx.SetJSONIfPresent(body, inputs, "extra_data", "extra_data"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if len(body) == 0 {
		return awx.ErrorResult("Nothing to update — set Enabled, or fill in at least one of Schedule Name, Description, Recurrence Rule or Extra Variables."), nil
	}

	// A new rule is validated by AWX before the change is made, so a typo cannot
	// leave the schedule in a state the operator did not ask for. AWX's own words
	// come back on a bad rule.
	if rrule := awx.OptionalString("rrule", inputs); rrule != "" {
		if _, err := awx.CreateResource(ctx, auth, "schedules/preview/", map[string]interface{}{"rrule": rrule}); err != nil {
			return awx.ErrorResult(fmt.Sprintf(
				"AWX would not accept that recurrence rule: %s "+
					"A rule needs a time-zone-aware DTSTART and an INTERVAL, for example "+
					"DTSTART;TZID=Europe/London:20260801T090000 RRULE:FREQ=DAILY;INTERVAL=1. The schedule has not been changed.",
				err.Error())), nil
		}
	}

	schedule, err := awx.UpdateResource(ctx, auth, fmt.Sprintf("schedules/%d/", scheduleID), body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	enabled := awx.BoolField(schedule, "enabled")
	state := "paused"
	if enabled {
		state = "enabled"
	}
	summary := fmt.Sprintf("Updated schedule %q (ID %d) — now %s", awx.StringField(schedule, "name"), scheduleID, state)
	if nextRun := awx.StringField(schedule, "next_run"); nextRun != "" && enabled {
		summary += ", next run " + nextRun
	}

	out := awx.ObjectResult(schedule, summary)
	out["enabled"] = enabled
	out["next_run"] = awx.StringField(schedule, "next_run")
	return out, nil
}
