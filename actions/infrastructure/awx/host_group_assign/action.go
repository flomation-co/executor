// Package infrastructure_awx_host_group_assign puts a host into an inventory
// group, or takes it out again.
//
// One action rather than two, because add and remove are literally the same AWX
// endpoint — POST /groups/{group_id}/hosts/ — separated only by one key in the
// body. That single key is the most dangerous thing in this node, so both traps
// are spelled out here.
//
// ★ TRAP 1 — THE ENDPOINT. This MUST be /groups/{id}/hosts/ and never
// /inventories/{id}/hosts/. They look interchangeable; they are not. The
// inventory sublist sets AWX's `parent_key`, and on a parent_key sublist a
// disassociate is not "remove from the list" — it is relationship.remove(), which
// for a parent_key relation means DELETE THE OBJECT. Removing a host from a group
// via the inventory sublist would HARD-DELETE the host. /groups/{id}/hosts/ sets
// no parent_key, so disassociate genuinely just breaks the membership and the
// host survives — which is why this action is NOT gated on confirm_destructive.
//
// ★ TRAP 2 — DISASSOCIATE IS A PRESENCE CHECK. AWX's attach handler tests
// `if 'disassociate' in request.data` — the KEY, not its value. So
// {"id": 5, "disassociate": false} STILL DISASSOCIATES. Sending the flag as false
// on an add would silently do the exact opposite of what the operator asked for.
// buildBody therefore constructs the map so the key is ABSENT on add, and
// TestAddBodyOmitsDisassociateEntirely pins it.
//
// AWX answers both operations with 204 and an EMPTY BODY — not the object, not a
// 200 — so nothing is parsed off the response. Attach is idempotent: adding a host
// already in the group is a no-op 204, as is removing one that is not in it.
package infrastructure_awx_host_group_assign

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Add or Remove Host from Group"
	Description  = "Put a host into an inventory group, or take it out again. Removing a host from a group does not delete the host."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+link"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

const (
	operationAdd    = "add"
	operationRemove = "remove"
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "The inventory the group and host belong to", Required: true},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "The group to add the host to, or take it out of", Required: true},
	{Name: "host_id", Type: core.ConnectionTypeString, Label: "Host", Placeholder: "The host to move", Required: true},
	{Name: "operation", Type: core.ConnectionTypeString, Label: "Operation", Placeholder: "Add the host to the group, or remove it from the group", Required: true, Options: []core.ConnectionOption{
		{Name: "Add to group", Value: operationAdd},
		{Name: "Remove from group", Value: operationRemove},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "host_id", Type: core.ConnectionTypeString, Label: "Host ID"},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group ID"},
	{Name: "operation", Type: core.ConnectionTypeString, Label: "Operation"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// buildBody assembles the attach/detach payload.
//
// ★ On an add, the "disassociate" key is not merely false — it IS NOT THERE. AWX
// checks for the key's PRESENCE, so a false value would remove the host. Do not
// "simplify" this into body["disassociate"] = (operation == operationRemove).
func buildBody(hostID int64, operation string) map[string]interface{} {
	body := map[string]interface{}{"id": hostID}
	if operation == operationRemove {
		body["disassociate"] = true
	}
	return body
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	// The inventory is not part of the request — a group id and a host id are
	// globally unique. It is required so the editor can scope the two dropdowns
	// below, and so the operator is looking at the inventory they think they are.
	if _, err := awx.RequiredInt("inventory_id", "Inventory", inputs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	groupID, err := awx.RequiredInt("group_id", "Group", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	hostID, err := awx.RequiredInt("host_id", "Host", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	operation, err := awx.RequiredString("operation", "Operation", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if operation != operationAdd && operation != operationRemove {
		return awx.ErrorResult(fmt.Sprintf("Operation must be either %q (add the host to the group) or %q (take it out of the group) — got %q", operationAdd, operationRemove, operation)), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	// /groups/{id}/hosts/ — NEVER /inventories/{id}/hosts/. See trap 1 above.
	if _, err := awx.CreateResource(ctx, auth, fmt.Sprintf("groups/%d/hosts/", groupID), buildBody(hostID, operation)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Added host %d to group %d", hostID, groupID)
	if operation == operationRemove {
		summary = fmt.Sprintf("Removed host %d from group %d — the host itself still exists", hostID, groupID)
	}
	return awx.SuccessResult(summary, map[string]interface{}{
		"host_id":   awx.IDString(hostID),
		"group_id":  awx.IDString(groupID),
		"operation": operation,
	}), nil
}
