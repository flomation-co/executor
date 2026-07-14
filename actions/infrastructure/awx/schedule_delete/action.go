// Package infrastructure_awx_schedule_delete permanently deletes an AWX schedule.
//
// DESTRUCTIVE, so it carries the confirm_destructive guard last. To stop a
// schedule from firing WITHOUT losing it, use "AWX: Update Schedule" with Enabled
// unticked — that is almost always what an operator actually wants, and the error
// message below says so.
package infrastructure_awx_schedule_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	awx "flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "AWX: Delete Schedule"
	Description  = "Permanently delete a schedule. To stop a schedule without losing it, use Update Schedule with Enabled off instead."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+trash"
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

	// ---- WHAT TO DELETE ----
	{Name: "schedule_id", Type: core.ConnectionTypeString, Label: "Schedule", Placeholder: "The schedule to delete", Required: true},

	// ---- THE GUARD (last, required) ----
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes AWX state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Schedule ID"},
	{Name: "deleted", Type: core.ConnectionTypeBoolean, Label: "Deleted"},
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

	// Fails closed: unset, blank or unparseable refuses the delete.
	if err := awx.ConfirmDestructive(inputs, fmt.Sprintf("delete AWX schedule %d", scheduleID)); err != nil {
		return awx.ErrorResult(err.Error() + ". (To stop a schedule without losing it, use “AWX: Update Schedule” with Enabled off.)"), nil
	}

	if _, err := awx.DeleteResource(ctx, auth, fmt.Sprintf("schedules/%d/", scheduleID)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.SuccessResult(fmt.Sprintf("Deleted AWX schedule %d", scheduleID), map[string]interface{}{
		"id":      awx.IDString(scheduleID),
		"deleted": true,
	}), nil
}
