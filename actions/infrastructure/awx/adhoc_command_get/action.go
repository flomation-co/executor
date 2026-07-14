// Package infrastructure_awx_adhoc_command_get fetches one AWX ad-hoc command.
//
// An ad-hoc command is one of AWX's five "unified job" kinds, so this could in
// principle be done with Get Job (job_kind = Ad-Hoc Command). It exists as its own
// action because an operator who ran an ad-hoc command is looking for what they
// ran — module_name and module_args — and those fields exist on no other job kind.
//
// The ad-hoc model is a SUBSET of the job model: there are no artifacts, no
// playbook, no project, no scm_revision, no job_template and no description on it.
// Emitting those as empty strings would invite a flow to branch on a field that
// can never be populated, so they are not emitted at all.
package infrastructure_awx_adhoc_command_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Ad-Hoc Command"
	Description  = "Fetch one ad-hoc command run — its status, per-host results and output."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+eye"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// --- AUTH BLOCK (verbatim from awx.AuthInputs) ---
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

	{Name: "adhoc_command_id", Type: core.ConnectionTypeString, Label: "Ad-Hoc Command ID", Placeholder: "The ID of the ad-hoc command, e.g. 42 — Run Ad-Hoc Command returns it", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ad-Hoc Command ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
	{Name: "failed", Type: core.ConnectionTypeBoolean, Label: "Failed"},
	{Name: "host_status_counts", Type: core.ConnectionTypeObject, Label: "Host Results"},
	{Name: "module_name", Type: core.ConnectionTypeString, Label: "Module"},
	{Name: "module_args", Type: core.ConnectionTypeString, Label: "Module Arguments"},
	{Name: "job_url", Type: core.ConnectionTypeString, Label: "AWX Link"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Ad-Hoc Command"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := awx.RequiredInt("adhoc_command_id", "Ad-Hoc Command ID", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	obj, err := awx.GetResource(ctx, auth, fmt.Sprintf("ad_hoc_commands/%d/", id), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	module := awx.StringField(obj, "module_name")
	status := awx.StringField(obj, "status")

	out := awx.ObjectResult(obj, fmt.Sprintf("Ad-hoc command %d (%s) is %s", id, module, status))
	out["status"] = status
	// `finished` is a TIMESTAMP on the AWX record — non-null means terminal. It is
	// exposed as a boolean because that is what a flow's condition branches on.
	out["finished"] = awx.StringField(obj, "finished") != ""
	out["failed"] = awx.BoolField(obj, "failed")
	out["host_status_counts"] = obj["host_status_counts"]
	out["module_name"] = module
	out["module_args"] = awx.StringField(obj, "module_args")
	out["job_url"] = awx.JobURL(auth, awx.JobKindAdHocCommand, id)
	return out, nil
}
