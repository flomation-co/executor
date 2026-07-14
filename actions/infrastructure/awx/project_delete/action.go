// Package infrastructure_awx_project_delete permanently deletes an AWX project.
//
// AWX answers 204 on success and 409 with an active_jobs envelope when something
// is still running against the project — awx.CheckResponse turns that 409 into a
// retryable "a job is still running against this resource" rather than a generic
// failure.
package infrastructure_awx_project_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Delete Project"
	Description  = "Permanently delete a project from AWX. Job templates that use it will stop working."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+trash"
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

	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "Choose the project to delete, or enter its AWX ID", Required: true},

	// LAST and Required, on every destructive AWX action.
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes AWX state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Project ID"},
	{Name: "deleted", Type: core.ConnectionTypeBoolean, Label: "Deleted"},
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
	if err := awx.ConfirmDestructive(inputs, fmt.Sprintf("delete project %d", projectID)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	if _, err := awx.DeleteResource(ctx, auth, fmt.Sprintf("projects/%d/", projectID)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.SuccessResult(fmt.Sprintf("Deleted project %d", projectID), map[string]interface{}{
		"id":      awx.IDString(projectID),
		"deleted": true,
	}), nil
}
