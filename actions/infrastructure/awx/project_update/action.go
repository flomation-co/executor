// Package infrastructure_awx_project_update changes a project's source-control
// settings.
//
// ★ PATCH, NEVER PUT. AWX copies each model field's default onto the serializer
// field, so a PUT that omits a field RESETS it to the model default — a PUT is a
// genuine destructive full-replace. awx.UpdateResource only ever PATCHes.
//
// This action still carries the confirm_destructive guard: it edits a shared,
// live resource, and pointing an existing project at a different repository or
// branch silently changes what EVERY job template using it runs.
package infrastructure_awx_project_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Update Project"
	Description  = "Change a project's source-control settings."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+pencil"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// --- AWX credential block (identical in all 59 AWX actions) ---
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

	// --- what to change. Every field is optional: only the ones you fill in are
	// sent, and everything else on the project is left exactly as it was.
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "Choose the project to change, or enter its AWX ID", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Leave blank to keep the current name"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Leave blank to keep the current description"},
	{Name: "scm_url", Type: core.ConnectionTypeString, Label: "Source Control URL", Placeholder: "https://github.com/acme/playbooks.git — leave blank to keep the current URL"},
	{Name: "scm_branch", Type: core.ConnectionTypeString, Label: "Branch / Tag / Commit", Placeholder: "main — leave blank to keep the current branch"},
	{Name: "credential_id", Type: core.ConnectionTypeString, Label: "Source Control Credential", Placeholder: "A Source Control credential — leave blank to keep the current one"},
	{Name: "scm_update_on_launch", Type: core.ConnectionTypeBoolean, Label: "Update On Launch", Placeholder: "Sync from source control every time a job template using this project is launched"},
	{Name: "allow_override", Type: core.ConnectionTypeBoolean, Label: "Allow job templates to override the branch"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"scm_clean": true} — any other AWX project field; these override the fields above`},

	// LAST and Required, on every destructive AWX action.
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes AWX state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Project ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Project"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err // the ONE hard failure: the node is mis-configured
	}

	ctx, cancel := awx.Context()
	defer cancel()

	projectID, err := awx.RequiredInt("project_id", "Project", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.ConfirmDestructive(inputs, fmt.Sprintf("change project %d", projectID)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	awx.SetIfPresent(body, inputs, "name", "name")
	awx.SetIfPresent(body, inputs, "description", "description")
	awx.SetIfPresent(body, inputs, "scm_url", "scm_url")
	awx.SetIfPresent(body, inputs, "scm_branch", "scm_branch")
	awx.SetBoolIfSet(body, inputs, "scm_update_on_launch", "scm_update_on_launch")
	awx.SetBoolIfSet(body, inputs, "allow_override", "allow_override")
	if err := awx.SetIntIfPresent(body, inputs, "credential", "credential_id"); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// LAST, so a power user's raw field wins over the first-class inputs above.
	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	if len(body) == 0 {
		return awx.ErrorResult("Nothing to change — fill in at least one field (Name, Description, Source Control URL, Branch, Credential, Update On Launch, Allow Override, or Additional Fields)."), nil
	}

	project, err := awx.UpdateResource(ctx, auth, fmt.Sprintf("projects/%d/", projectID), body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.ObjectResult(project, fmt.Sprintf("Updated project %q (%d)",
		awx.StringField(project, "name"), projectID)), nil
}
