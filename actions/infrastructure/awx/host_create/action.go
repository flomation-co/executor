// Package infrastructure_awx_host_create adds a host to an inventory.
//
// ★ THE `enabled` TRI-STATE. AWX defaults Host.enabled to TRUE, but the manifest
// cannot carry a default value, so the editor renders the checkbox UNTICKED. If
// an untouched checkbox sent enabled=false, every host created by this node would
// silently be disabled and would be skipped by every job that targets it — a bug
// nobody would find until a playbook mysteriously ran against nothing. So the
// field is sent ONLY when the operator actually set it (SetBoolIfSet): untouched
// means "omit, and let AWX default it to true".
package infrastructure_awx_host_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Create Host"
	Description  = "Add a host to an inventory."
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "The inventory to add the host to", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Host Name", Placeholder: "web01.example.com", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this host is for"},
	{Name: "enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled", Placeholder: "Leave unticked to let AWX decide (new hosts are enabled by default). Tick to force it enabled."},
	{Name: "instance_id", Type: core.ConnectionTypeString, Label: "Instance ID", Placeholder: "Optional cloud instance id, e.g. i-0abc123 — used to match this host against a cloud inventory source"},
	{Name: "variables", Type: core.ConnectionTypeText, Label: "Host Variables", Placeholder: "YAML or JSON, e.g. ansible_host: 10.0.0.5"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `Any other AWX host field, as JSON — e.g. {"instance_id":"i-0abc123"}. Takes precedence over the fields above.`},
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
	inventoryID, err := awx.RequiredInt("inventory_id", "Inventory", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	name, err := awx.RequiredString("name", "Host Name", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"inventory": inventoryID,
		"name":      name,
	}
	awx.SetIfPresent(body, inputs, "description", "description")
	// Tri-state — see the package comment. NEVER body["enabled"] = BoolInput(...).
	awx.SetBoolIfSet(body, inputs, "enabled", "enabled")
	awx.SetIfPresent(body, inputs, "instance_id", "instance_id")
	// AWX takes host variables as a YAML-or-JSON STRING, not as a nested object.
	awx.SetIfPresent(body, inputs, "variables", "variables")
	if err := awx.MergeAdditionalFields(body, inputs); err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	host, err := awx.CreateResource(ctx, auth, "hosts/", body)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}
	return awx.ObjectResult(host, fmt.Sprintf("Created host %s in inventory %d", name, inventoryID)), nil
}
