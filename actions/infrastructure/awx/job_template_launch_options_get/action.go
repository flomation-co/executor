// Package infrastructure_awx_job_template_launch_options_get asks a job template
// what it will let you set at launch.
//
// This is the same pre-flight the Launch Job Template action runs internally,
// exposed as an action of its own so an AI tool loop — or a flow that builds a
// form for a human — can introspect a template BEFORE launching it, rather than
// discovering AWX silently dropped an override after the playbook has already run
// against every host.
//
// Three shape traps, all handled here:
//
//   - can_start_without_user_input is false merely because prompting is
//     AVAILABLE, not because input is MANDATORY. It is NOT "launch will fail" —
//     the summary says so in words, because reading it as a failure predicate is
//     the obvious mistake.
//   - credential_needed_to_start is hardcoded False in AWX (legacy) — ignored.
//   - variables_needed_to_start means two different things in AWX. HERE it is the
//     list of required survey VARIABLE NAMES; in a 400 body it is a list of
//     human-readable ERROR STRINGS. Never parse the two with one code path.
package infrastructure_awx_job_template_launch_options_get

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Job Template Launch Options"
	Description  = "Ask a job template what it will let you set at launch — which fields it prompts for, which survey answers are required, and whether it needs a password."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+magnifying-glass"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// ---- AUTH (the shared block — see awx.AuthInputs) ----
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

	{Name: "job_template_id", Type: core.ConnectionTypeString, Label: "Job Template", Placeholder: "Pick a job template, or enter its AWX ID (e.g. 7)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Launch Options"},
	{Name: "can_start_without_user_input", Type: core.ConnectionTypeBoolean, Label: "Can Start Without Input"},
	{Name: "promptable_fields", Type: core.ConnectionTypeObject, Label: "Promptable Fields"},
	{Name: "variables_needed_to_start", Type: core.ConnectionTypeObject, Label: "Required Survey Variables"},
	{Name: "passwords_needed_to_start", Type: core.ConnectionTypeObject, Label: "Required Passwords"},
	{Name: "survey_enabled", Type: core.ConnectionTypeBoolean, Label: "Has Survey"},
	{Name: "inventory_needed_to_start", Type: core.ConnectionTypeBoolean, Label: "Needs an Inventory"},
	{Name: "defaults", Type: core.ConnectionTypeObject, Label: "Defaults"},
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

	id, err := awx.RequiredInt("job_template_id", "Job Template", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	cfg, err := awx.PreflightLaunch(ctx, auth, awx.TemplateKindJob, id)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	raw := cfg.Raw
	if raw == nil {
		raw = map[string]interface{}{}
	}
	promptable := cfg.PromptableFields()

	return map[string]interface{}{
		"result":                       raw,
		"can_start_without_user_input": cfg.CanStartWithoutUserInput,
		// The 16 ask_*_on_launch flags reduced to the BODY-FIELD names that are on
		// — i.e. exactly the fields the Launch Job Template node will accept and
		// AWX will honour. Anything else it would silently drop.
		"promptable_fields":         strings2iface(promptable),
		"variables_needed_to_start": strings2iface(cfg.VariablesNeededToStart),
		"passwords_needed_to_start": strings2iface(cfg.PasswordsNeededToStart),
		"survey_enabled":            cfg.SurveyEnabled,
		"inventory_needed_to_start": cfg.InventoryNeededToStart,
		"defaults":                  cfg.Defaults,
		"tool_result":               summarise(id, cfg, promptable),
		"success":                   true,
		"error":                     "",
	}, nil
}

func summarise(id int64, cfg awx.LaunchConfig, promptable []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Job template %d: ", id)

	if len(promptable) > 0 {
		fmt.Fprintf(&b, "will accept these overrides at launch — %s. Anything else you set on the Launch node would be SILENTLY IGNORED by AWX and the job would run with the template's own values.", strings.Join(promptable, ", "))
	} else {
		b.WriteString("prompts for NOTHING at launch. Leave every override on the Launch node blank — AWX would silently ignore them and run with the template's own values.")
	}

	if cfg.SurveyEnabled {
		b.WriteString(" It has a survey.")
		if len(cfg.VariablesNeededToStart) > 0 {
			fmt.Fprintf(&b, " These survey variables have no default and MUST be answered: %s.", strings.Join(cfg.VariablesNeededToStart, ", "))
		} else {
			b.WriteString(" Every survey question has a default, so it can be launched without answering any of them.")
		}
	}

	if len(cfg.PasswordsNeededToStart) > 0 {
		fmt.Fprintf(&b, " Its credentials will ask for a password at launch — fill in Credential Passwords with: %s.", strings.Join(cfg.PasswordsNeededToStart, ", "))
	}

	if cfg.InventoryNeededToStart {
		if cfg.Ask["inventory"] {
			b.WriteString(" It has no inventory of its own, so you must choose one on the Launch node.")
		} else {
			b.WriteString(" ⚠ It has no inventory and does not prompt for one, so it cannot be launched at all until an inventory is set on the template in AWX.")
		}
	}

	// The mistake this sentence exists to prevent: can_start_without_user_input is
	// false whenever prompting is merely AVAILABLE. It is not a failure predicate.
	if !cfg.CanStartWithoutUserInput {
		b.WriteString(" (AWX reports can_start_without_user_input=false. That only means prompting is available — it does NOT mean the launch will fail.)")
	}
	return b.String()
}

// strings2iface widens a []string to the []interface{} the flow engine's Loop
// node iterates. An empty list stays an empty ARRAY, never null.
func strings2iface(in []string) []interface{} {
	out := make([]interface{}, 0, len(in))
	for _, s := range in {
		out = append(out, s)
	}
	return out
}
