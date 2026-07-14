// Package infrastructure_awx_group_delete permanently deletes an AWX inventory
// group.
//
// ★ THIS IS THE MOST SURPRISING DELETION IN AWX, and the reason this action's
// description, its confirm placeholder and its summary all spell the consequence
// out. GroupDetail.destroy does not delete one row — it calls
// Group.delete_recursive(), which deletes:
//
//   - the group;
//   - every DESCENDANT group that is left without a parent as a result; and
//   - every HOST that would be left in no group at all.
//
// So deleting a group can delete hosts. A non-technical operator will read
// "delete group" as "un-file these machines", not "destroy them", so the guard is
// required and the wording says what actually happens. As ever the guard is a
// boolean, so a flow can bind it to ${var.approved} from a Human-in-the-Loop node
// and decide at run time rather than at design time.
//
// AWX answers 204 on success and 409 while a job is still running against the
// inventory; awx.CheckResponse turns that 409 into "wait for it to finish".
package infrastructure_awx_group_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Delete Group"
	Description  = "Permanently delete an inventory group. WARNING: AWX deletes this recursively — child groups that are left without a parent go too, and any host left in no group at all is DELETED."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+trash"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// ---- AUTH BLOCK (awx.AuthInputs, verbatim — see awx_inputs_drift_test.go) ----
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
	// ---- END AUTH BLOCK ----

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Pick the inventory the group lives in — this is what fills the Group list below"},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "The group to delete", Required: true},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "Deleting a group ALSO deletes its orphaned child groups and any host left in no group. Tick to allow, or bind ${var.approved}.", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group ID"},
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
	groupID, err := awx.RequiredInt("group_id", "Group", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if err := awx.ConfirmDestructive(inputs,
		fmt.Sprintf("delete group %d — which also deletes any child group left without a parent, and any host left in no group at all", groupID)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	if _, err := awx.DeleteResource(ctx, auth, fmt.Sprintf("groups/%d/", groupID)); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	id := awx.IDString(groupID)
	return awx.SuccessResult(
		fmt.Sprintf("Deleted group %s — along with any child group left without a parent, and any host that was left in no group at all", id),
		map[string]interface{}{
			"id":      id,
			"deleted": true,
		}), nil
}
