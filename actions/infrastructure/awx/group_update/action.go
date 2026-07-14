// Package infrastructure_awx_group_update renames an AWX inventory group or
// changes its variables.
//
// Two properties are load-bearing:
//
//   - It PATCHes, never PUTs. AWX's serializers copy each model field's default
//     onto the field, so a PUT that omits `variables` would RESET them — a silent
//     data loss. Only the fields the operator actually filled in are sent.
//   - It never sends `inventory`. Group.inventory is read-only after creation (as
//     Host.inventory is); a group cannot be moved between inventories, and AWX
//     would reject the attempt. Inventory is on this node purely to scope the
//     Group picker.
package infrastructure_awx_group_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Update Group"
	Description  = "Rename an inventory group or change its variables."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+pencil"
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
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "The group to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Group Name", Placeholder: "New name — leave blank to keep the current one"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description — leave blank to keep the current one"},
	{Name: "variables", Type: core.ConnectionTypeText, Label: "Variables", Placeholder: "Group variables as YAML or JSON — REPLACES the group's existing variables. Leave blank to keep them."},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `Any other AWX group field to change — these override the fields above`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Group"},
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

	body := map[string]interface{}{}
	awx.SetIfPresent(body, inputs, "name", "name")
	awx.SetIfPresent(body, inputs, "description", "description")
	awx.SetIfPresent(body, inputs, "variables", "variables")
	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	if len(body) == 0 {
		// An empty PATCH would answer 200 and change nothing, reporting a success
		// the operator never got. Say so instead.
		return awx.ErrorResult("Nothing to update — fill in Group Name, Description, Variables or Additional Fields."), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	obj, err := awx.UpdateResource(ctx, auth, fmt.Sprintf("groups/%d/", groupID), body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	name := awx.StringField(obj, "name")
	if name == "" {
		name = awx.IDString(groupID)
	}
	return awx.ObjectResult(obj, fmt.Sprintf("Updated group %s", name)), nil
}
