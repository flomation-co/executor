// Package infrastructure_awx_inventory_update renames an AWX inventory or
// changes its description, variables or host filter.
//
// It PATCHes. A PUT would be a full replace, and AWX copies every model default
// onto the serializer — so a PUT that omitted a field would RESET that field
// rather than leave it alone.
//
// `kind` is deliberately not an input: AWX refuses to change it after creation
// (405, "You cannot turn a regular inventory into a smart or constructed
// inventory"), so offering the field would only produce a confusing failure.
// Delete and re-create is the only way. `variables` is sent as a STRING for the
// same reason as in inventory_create — the model field is a TextField, and a real
// JSON object is answered with 400 "Not a valid string."
package infrastructure_awx_inventory_update

import (
	"errors"
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Update Inventory"
	Description  = "Rename an inventory or change its variables."
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "The inventory to change", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Leave blank to keep the current name"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Leave blank to keep the current description"},
	{Name: "variables", Type: core.ConnectionTypeText, Label: "Variables", Placeholder: "YAML or JSON, e.g. ansible_user: deploy — REPLACES the inventory's current variables"},
	{Name: "host_filter", Type: core.ConnectionTypeString, Label: "Host Filter", Placeholder: "name__icontains=web — only meaningful on a smart inventory"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"prevent_instance_group_fallback":true} — any other AWX inventory field; these override the fields above`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Inventory ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Inventory"},
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

	body := map[string]interface{}{}
	awx.SetIfPresent(body, inputs, "name", "name")
	awx.SetIfPresent(body, inputs, "description", "description")
	awx.SetIfPresent(body, inputs, "variables", "variables")
	awx.SetIfPresent(body, inputs, "host_filter", "host_filter")
	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// An empty PATCH would answer 200 with the object unchanged, so the node would
	// report a cheerful success having done nothing at all. Say so instead.
	if len(body) == 0 {
		return awx.ErrorResult(errors.New("nothing to update — fill in at least one of Name, Description, Variables or Host Filter").Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	obj, err := awx.UpdateResource(ctx, auth, fmt.Sprintf("inventories/%d/", id), body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.ObjectResult(obj, fmt.Sprintf("Updated inventory %q (ID %d)", awx.StringField(obj, "name"), id)), nil
}
