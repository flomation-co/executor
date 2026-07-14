// Package infrastructure_awx_group_create creates a group inside an AWX
// inventory.
//
// Three AWX rejections are re-worded here, because the raw 400s are all but
// unreadable to a non-technical operator:
//
//   - "Invalid group name." — AWX reserves `all` and `_meta` (they mean something
//     to Ansible's inventory format itself).
//   - "A Host with that name already exists." — a group may not share its name
//     with a HOST in the same inventory, which is a rule nobody expects.
//   - a smart or constructed inventory cannot hold groups at all; its contents are
//     computed from a host filter, so there is nothing to put a group into.
package infrastructure_awx_group_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Create Group"
	Description  = "Create a group in an inventory to organise hosts."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+plus"
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "The inventory to create the group in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Group Name", Placeholder: "e.g. webservers — cannot be 'all' or '_meta', and cannot match a host name in this inventory", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this group is for"},
	{Name: "variables", Type: core.ConnectionTypeText, Label: "Variables", Placeholder: "Group variables as YAML or JSON, e.g. http_port: 8080"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `Any other AWX group field, e.g. {"description":"…"} — these override the fields above`},
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
	inventoryID, err := awx.RequiredInt("inventory_id", "Inventory", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	name, err := awx.RequiredString("name", "Group Name", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"inventory": inventoryID,
		"name":      name,
	}
	awx.SetIfPresent(body, inputs, "description", "description")
	// AWX takes group variables as a STRING of YAML or JSON, not as a JSON object,
	// so this is passed straight through rather than parsed.
	awx.SetIfPresent(body, inputs, "variables", "variables")
	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	obj, err := awx.CreateResource(ctx, auth, "groups/", body)
	if err != nil {
		return awx.ErrorResult(explain(err.Error(), name)), nil
	}

	return awx.ObjectResult(obj, fmt.Sprintf("Created group %s (ID %s)", name, awx.IDString(obj["id"]))), nil
}

// explain re-words the three AWX 400s an operator hits when creating a group. The
// raw messages name Ansible internals ("_meta") or contradict themselves ("a Host
// with that name already exists" when you are creating a GROUP), so each is
// replaced with what the operator has to actually do about it.
func explain(msg, name string) string {
	switch {
	case strings.Contains(msg, "Invalid group name"):
		return fmt.Sprintf("AWX will not accept %q as a group name — 'all' and '_meta' are reserved by Ansible's inventory format. Choose another name.", name)

	case strings.Contains(msg, "A Host with that name already exists"):
		return fmt.Sprintf("A HOST called %q already exists in this inventory, and AWX does not allow a group and a host to share a name. Rename the group.", name)

	case strings.Contains(msg, "Cannot create Group for Smart Inventory"),
		strings.Contains(msg, "Cannot create Group for Constructed Inventory"),
		strings.Contains(msg, "Smart Inventory"),
		strings.Contains(msg, "Constructed Inventory"):
		return "This is a smart or constructed inventory — its contents are computed from a host filter, so it cannot hold groups. Create the group in a standard inventory instead."
	}
	return msg
}
