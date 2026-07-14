// Package infrastructure_awx_job_template_get fetches one AWX / AAP job
// template.
//
// Read only. There is deliberately no create/update/delete on job templates in
// this node: a non-technical operator should LAUNCH templates an Ansible
// engineer authored, not author them from a flow. Read + launch is the whole
// surface.
package infrastructure_awx_job_template_get

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Job Template"
	Description  = "Fetch one job template — its playbook, project, inventory, and which fields it will let you override at launch."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+eye"
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
	{Name: "id", Type: core.ConnectionTypeString, Label: "Job Template ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Job Template"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "playbook", Type: core.ConnectionTypeString, Label: "Playbook"},
	{Name: "survey_enabled", Type: core.ConnectionTypeBoolean, Label: "Has Survey"},
	{Name: "can_launch", Type: core.ConnectionTypeBoolean, Label: "Can Launch"},
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

	obj, err := awx.GetResource(ctx, auth, fmt.Sprintf("job_templates/%d/", id), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	canLaunch := canStart(obj)

	out := awx.ObjectResult(obj, summarise(obj, canLaunch))
	out["name"] = awx.StringField(obj, "name")
	out["playbook"] = awx.StringField(obj, "playbook")
	out["survey_enabled"] = awx.BoolField(obj, "survey_enabled")
	out["can_launch"] = canLaunch
	return out, nil
}

func summarise(obj map[string]interface{}, canLaunch bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Job template %s %q", awx.IDString(obj["id"]), awx.StringField(obj, "name"))
	if playbook := awx.StringField(obj, "playbook"); playbook != "" {
		fmt.Fprintf(&b, " runs %s", playbook)
	}
	b.WriteString(".")

	if awx.BoolField(obj, "survey_enabled") {
		b.WriteString(" It has a survey — use Get Job Template Survey to see the questions it will ask at launch.")
	}

	// The prompt flags are the operator's real question ("can I set a limit on
	// this?"), and getting it wrong is the node's headline hazard: AWX silently
	// DROPS an override a template does not prompt for.
	if prompts := promptableFields(obj); len(prompts) > 0 {
		fmt.Fprintf(&b, " It will let you override %s at launch; anything else you set would be silently ignored by AWX.", strings.Join(prompts, ", "))
	} else {
		b.WriteString(" It prompts for nothing at launch — any override you set on the Launch node would be silently ignored by AWX, so leave them blank.")
	}

	if canLaunch {
		b.WriteString(" This AWX user may launch it.")
	} else {
		b.WriteString(" ⚠ This AWX user may NOT launch it — the API token is read-scoped, or the user has no Execute role on it — so launching would fail with a 403.")
	}
	return b.String()
}

// askFieldLabels maps AWX's ask_*_on_launch flags to the wording AWX's own UI
// uses, so the summary names the checkbox an operator would have to tick.
//
// Three of the sixteen do NOT follow the ask_X_on_launch → X rule
// (ask_variables → Variables, ask_tags → Job Tags, ask_credential → Credentials),
// which is why this is a table rather than a string trim.
var askFieldLabels = []struct{ flag, label string }{
	{"ask_credential_on_launch", "Credentials"},
	{"ask_diff_mode_on_launch", "Show Changes"},
	{"ask_execution_environment_on_launch", "Execution Environment"},
	{"ask_forks_on_launch", "Forks"},
	{"ask_instance_groups_on_launch", "Instance Groups"},
	{"ask_inventory_on_launch", "Inventory"},
	{"ask_job_slice_count_on_launch", "Job Slicing"},
	{"ask_job_type_on_launch", "Job Type"},
	{"ask_labels_on_launch", "Labels"},
	{"ask_limit_on_launch", "Limit"},
	{"ask_scm_branch_on_launch", "Source Control Branch"},
	{"ask_skip_tags_on_launch", "Skip Tags"},
	{"ask_tags_on_launch", "Job Tags"},
	{"ask_timeout_on_launch", "Timeout"},
	{"ask_variables_on_launch", "Variables"},
	{"ask_verbosity_on_launch", "Verbosity"},
}

// promptableFields lists, in a stable order, the fields this template accepts at
// launch.
func promptableFields(obj map[string]interface{}) []string {
	out := []string{}
	for _, f := range askFieldLabels {
		if awx.BoolField(obj, f.flag) {
			out = append(out, f.label)
		}
	}
	return out
}

// canStart reads summary_fields.user_capabilities.start — AWX's own answer to
// "may the caller run this?". Absent reads as false, the safe direction.
func canStart(obj map[string]interface{}) bool {
	summary, _ := obj["summary_fields"].(map[string]interface{})
	caps, _ := summary["user_capabilities"].(map[string]interface{})
	return awx.BoolField(caps, "start")
}
