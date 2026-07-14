// Package infrastructure_awx_inventory_get fetches one AWX inventory.
//
// Besides the whole object it surfaces the four fields an operator actually
// branches on: how many hosts and groups it holds, whether any host is currently
// failing, and whether the inventory is already queued for deletion — AWX deletes
// an inventory asynchronously, so a row with pending_deletion=true still answers
// a GET with 200 while its hosts are being torn down in the background.
package infrastructure_awx_inventory_get

import (
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Inventory"
	Description  = "Fetch one inventory — its host and group counts and whether any hosts are failing."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+eye"
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "The inventory to fetch", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Inventory ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Inventory"},
	{Name: "total_hosts", Type: core.ConnectionTypeInteger, Label: "Hosts"},
	{Name: "total_groups", Type: core.ConnectionTypeInteger, Label: "Groups"},
	{Name: "has_active_failures", Type: core.ConnectionTypeBoolean, Label: "Has Failing Hosts"},
	{Name: "pending_deletion", Type: core.ConnectionTypeBoolean, Label: "Pending Deletion"},
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

	ctx, cancel := awx.Context()
	defer cancel()

	obj, err := awx.GetResource(ctx, auth, fmt.Sprintf("inventories/%d/", id), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	hosts := intField(obj, "total_hosts")
	groups := intField(obj, "total_groups")
	pending := awx.BoolField(obj, "pending_deletion")

	summary := fmt.Sprintf("Fetched inventory %q (%d hosts, %d groups)", awx.StringField(obj, "name"), hosts, groups)
	if pending {
		summary += " — this inventory is queued for deletion and will disappear shortly"
	}

	out := awx.ObjectResult(obj, summary)
	out["total_hosts"] = hosts
	out["total_groups"] = groups
	out["has_active_failures"] = awx.BoolField(obj, "has_active_failures")
	out["pending_deletion"] = pending
	return out, nil
}

// intField reads one of AWX's count fields. They arrive as JSON numbers, i.e.
// float64 — a plain assertion to int would yield 0 for every one of them.
func intField(obj map[string]interface{}, key string) int64 {
	switch v := obj[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}
