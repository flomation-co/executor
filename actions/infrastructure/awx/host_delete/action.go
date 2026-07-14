// Package infrastructure_awx_host_delete permanently removes a host from AWX.
//
// DESTRUCTIVE and not recoverable: the host, its variables and its entire job
// history on this host disappear. Gated on confirm_destructive, which fails
// closed — an unset, blank or unresolvable value refuses.
//
// To take a host OUT OF A GROUP without deleting it, use "AWX: Add or Remove Host
// from Group" instead. To stop a host being targeted without losing it, untick
// Enabled on "AWX: Update Host".
package infrastructure_awx_host_delete

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Delete Host"
	Description  = "Permanently remove a host from AWX. Requires explicit confirmation."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+trash"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Only used to populate the Host list below"},
	{Name: "host_id", Type: core.ConnectionTypeString, Label: "Host", Placeholder: "The host to delete — this cannot be undone", Required: true},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes AWX state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Host ID"},
	{Name: "deleted", Type: core.ConnectionTypeBoolean, Label: "Deleted"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	hostID, err := awx.RequiredInt("host_id", "Host", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.ConfirmDestructive(inputs, fmt.Sprintf("delete host %d from AWX", hostID)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	status, err := awx.DeleteResource(ctx, auth, fmt.Sprintf("hosts/%d/", hostID))
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	id := awx.IDString(hostID)
	summary := fmt.Sprintf("Deleted host %s", id)
	if status == http.StatusAccepted {
		// Hosts delete synchronously (204); an inventory does not. Defensive — if a
		// future AWX queues host deletion, say so rather than claiming it is gone.
		summary = fmt.Sprintf("AWX accepted the deletion of host %s — it is being removed in the background", id)
	}
	return awx.SuccessResult(summary, map[string]interface{}{
		"id":      id,
		"deleted": true,
	}), nil
}
