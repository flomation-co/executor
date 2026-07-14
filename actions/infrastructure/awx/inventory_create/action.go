// Package infrastructure_awx_inventory_create creates an AWX inventory.
//
// Two AWX rules shape this action:
//
//   - `variables` MUST be sent as a STRING containing YAML or JSON. The model
//     field is a TextField behind a CharNullField, so posting a real JSON object
//     is answered with 400 {"variables":["Not a valid string."]} — which is why
//     the input is a Text box whose contents are forwarded verbatim, and not an
//     Object. (A schedule's extra_data, by contrast, IS a real JSON object. The
//     inconsistency is AWX's.)
//
//   - kind="smart" REQUIRES a host filter. AWX answers 400 without one, so the
//     action refuses up-front with a message that names the field on the node.
package infrastructure_awx_inventory_create

import (
	"errors"
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Create Inventory"
	Description  = "Create an inventory in AWX to hold hosts and groups."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+plus"
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

	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Production Web Servers", Required: true},
	{Name: "organization_id", Type: core.ConnectionTypeString, Label: "Organization", Placeholder: "The organization that will own this inventory", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this inventory is for"},
	{Name: "kind", Type: core.ConnectionTypeString, Label: "Kind", Placeholder: "Standard unless you need a smart inventory — the kind cannot be changed later", Options: []core.ConnectionOption{
		{Name: "Standard", Value: ""},
		{Name: "Smart", Value: "smart"},
	}},
	{Name: "host_filter", Type: core.ConnectionTypeString, Label: "Host Filter", Placeholder: "name__icontains=web — required for a smart inventory", Visible: &core.VisibleWhen{Field: "kind", Values: []string{"smart"}}},
	{Name: "variables", Type: core.ConnectionTypeText, Label: "Variables", Placeholder: "YAML or JSON, e.g. ansible_user: deploy"},
	{Name: "prevent_instance_group_fallback", Type: core.ConnectionTypeBoolean, Label: "Prevent Instance Group Fallback", Placeholder: "Only run jobs on this inventory's own instance groups, never on the organization's or the global default"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"variables":"---"} — any other AWX inventory field; these override the fields above`},
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

	body, err := buildBody(inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	obj, err := awx.CreateResource(ctx, auth, "inventories/", body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	return awx.ObjectResult(obj, fmt.Sprintf("Created inventory %q (ID %s)",
		awx.StringField(obj, "name"), awx.IDString(obj["id"]))), nil
}

func buildBody(inputs []*core.Connection) (map[string]interface{}, error) {
	name, err := awx.RequiredString("name", "Name", inputs)
	if err != nil {
		return nil, err
	}
	organization, err := awx.RequiredInt("organization_id", "Organization", inputs)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"name":         name,
		"organization": organization,
	}
	awx.SetIfPresent(body, inputs, "description", "description")

	kind := awx.OptionalString("kind", inputs)
	hostFilter := awx.OptionalString("host_filter", inputs)
	if kind == "smart" && hostFilter == "" {
		return nil, errors.New("a Smart inventory needs a Host Filter — AWX has nothing to select hosts with otherwise. Set one, e.g. name__icontains=web, or change Kind to Standard")
	}
	if kind != "" {
		body["kind"] = kind
	}
	if hostFilter != "" {
		body["host_filter"] = hostFilter
	}

	// A STRING, deliberately — see the package comment. Whatever the operator
	// typed (YAML or JSON) is forwarded verbatim for AWX to parse.
	awx.SetIfPresent(body, inputs, "variables", "variables")

	// Tri-state: an untouched checkbox is omitted rather than sent as false, so it
	// cannot silently overwrite an AWX-side default.
	awx.SetBoolIfSet(body, inputs, "prevent_instance_group_fallback", "prevent_instance_group_fallback")

	// Last, so a power user's raw field wins over a first-class input.
	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	return body, nil
}
