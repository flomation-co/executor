// Package infrastructure_awx_host_update renames a host, rewrites its variables,
// or enables/disables it.
//
// ★ THE INVENTORY IS NOT SENT. Host.inventory becomes READ-ONLY once the host
// exists (AWX's serializer swaps the field to read_only when there is an
// instance), so a host cannot be moved between inventories and any `inventory`
// key on a PATCH is SILENTLY IGNORED — a 200 that changed nothing. The Inventory
// input here exists only to scope the Host live dropdown; it is never put in the
// body, so the operator is never told a move worked when it did not.
//
// ★ PATCH, NEVER PUT. AWX copies each model field's default onto the serializer,
// so a PUT that omits a field RESETS it. UpdateResource always PATCHes.
package infrastructure_awx_host_update

import (
	"errors"
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Update Host"
	Description  = "Rename a host, change its variables, or enable/disable it."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+pencil"
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Only used to populate the Host list below — a host cannot be moved between inventories"},
	{Name: "host_id", Type: core.ConnectionTypeString, Label: "Host", Placeholder: "The host to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Host Name", Placeholder: "Leave blank to keep the current name"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Leave blank to keep the current description"},
	{Name: "enabled", Type: core.ConnectionTypeString, Label: "Enabled", Placeholder: "Enable or disable the host — leave unchanged to keep its current state", Options: []core.ConnectionOption{
		{Name: "Leave unchanged", Value: ""},
		{Name: "Enabled", Value: "true"},
		{Name: "Disabled", Value: "false"},
	}},
	{Name: "variables", Type: core.ConnectionTypeText, Label: "Host Variables", Placeholder: "YAML or JSON, e.g. ansible_host: 10.0.0.5 — REPLACES all existing host variables"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `Any other AWX host field, as JSON — e.g. {"enabled":false}. Takes precedence over the fields above.`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Host ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Host"},
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

	body := map[string]interface{}{}
	awx.SetIfPresent(body, inputs, "name", "name")
	awx.SetIfPresent(body, inputs, "description", "description")
	awx.SetBoolIfSet(body, inputs, "enabled", "enabled")
	awx.SetIfPresent(body, inputs, "variables", "variables")
	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	// Note what is NOT here: `inventory`. The inventory_id input is never copied
	// into the body — AWX would take the PATCH, answer 200 and change nothing.
	// (Additional Fields remains a true escape hatch, so a power user who really
	// wants to try it still can; it just will not work, and that is AWX's answer,
	// not ours.)

	if len(body) == 0 {
		return awx.ErrorResult(errors.New("nothing to update — fill in at least one of Host Name, Description, Enabled, Host Variables or Additional Fields").Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	host, err := awx.UpdateResource(ctx, auth, fmt.Sprintf("hosts/%d/", hostID), body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	name := awx.StringField(host, "name")
	if name == "" {
		name = awx.IDString(hostID)
	}
	return awx.ObjectResult(host, fmt.Sprintf("Updated host %s", name)), nil
}
