// Package infrastructure_awx_inventory_delete deletes an AWX inventory.
//
// DESTRUCTIVE, and it cascades: every host and every group in the inventory goes
// with it, and any job template pointing at the inventory stops working. Hence
// the mandatory Confirm Destructive Action guard.
//
// ★ AWX DELETES AN INVENTORY ASYNCHRONOUSLY. The DELETE answers 202 ACCEPTED, not
// 204: the row is merely flagged pending_deletion=true, and a background task
// tears the hosts and groups down afterwards. An immediate GET therefore still
// answers 200 with the inventory in it — which is why this action reports
// pending_deletion and does NOT poll for a 404. Polling would pin a flow worker
// for as long as AWX took to finish, and on a large inventory that is minutes.
//
// A 409 means a job is still running against the inventory; AWX refuses to delete
// it until that job finishes or is cancelled. CheckResponse already renders that
// as "a job is still running against this resource", and it is retryable.
package infrastructure_awx_inventory_delete

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Delete Inventory"
	Description  = "Permanently delete an inventory and every host and group in it."
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "The inventory to delete — its hosts and groups go with it", Required: true},

	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This permanently changes AWX state. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Inventory ID"},
	{Name: "deleted", Type: core.ConnectionTypeBoolean, Label: "Deleted"},
	{Name: "pending_deletion", Type: core.ConnectionTypeBoolean, Label: "Deletion Still In Progress"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := awx.RequiredInt("inventory_id", "Inventory", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.ConfirmDestructive(inputs, fmt.Sprintf("delete AWX inventory %d and every host and group in it", id)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	status, err := awx.DeleteResource(ctx, auth, fmt.Sprintf("inventories/%d/", id))
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// 202 is the normal answer: AWX has ACCEPTED the deletion and queued it. 204
	// (a synchronous delete) is handled too, since a future AWX could return it.
	//
	// `deleted` means "AWX accepted the deletion", so a flow can branch on it
	// whichever status came back; `pending_deletion` is what tells the operator the
	// rows are still going away in the background and a GET will still find them
	// for a little while yet.
	pending := status == http.StatusAccepted

	summary := fmt.Sprintf("Deleted inventory %d", id)
	if pending {
		summary = fmt.Sprintf("AWX has accepted the deletion of inventory %d. It is being removed in the background along with its hosts and groups, so it may still appear in AWX for a short while.", id)
	}

	return awx.SuccessResult(summary, map[string]interface{}{
		"id":               fmt.Sprintf("%d", id),
		"deleted":          true,
		"pending_deletion": pending,
	}), nil
}
