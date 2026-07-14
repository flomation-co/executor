// Package infrastructure_awx_inventory_source_get fetches one inventory source
// and the status of its last sync.
package infrastructure_awx_inventory_source_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Inventory Source"
	Description  = "Fetch one inventory source and its last sync status."
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Choose the inventory the source belongs to — this narrows the Inventory Source list below"},
	{Name: "inventory_source_id", Type: core.ConnectionTypeString, Label: "Inventory Source", Placeholder: "The source to fetch", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Inventory Source ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Last Sync Status"},
	{Name: "last_updated", Type: core.ConnectionTypeString, Label: "Last Synced"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Inventory Source"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	sourceID, err := awx.RequiredInt("inventory_source_id", "Inventory Source", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	ctx, cancel := awx.Context()
	defer cancel()

	obj, err := awx.GetResource(ctx, auth, fmt.Sprintf("inventory_sources/%d/", sourceID), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	name := awx.StringField(obj, "name")
	status := awx.StringField(obj, "status")
	lastUpdated := awx.StringField(obj, "last_updated")

	summary := fmt.Sprintf("Inventory source %q (%s)", name, awx.IDString(obj["id"]))
	if source := awx.StringField(obj, "source"); source != "" {
		summary += " — source type " + source
	}
	switch {
	case status == "":
		summary += "; it has never been synced"
	case lastUpdated == "":
		summary += fmt.Sprintf("; last sync status %s", status)
	default:
		summary += fmt.Sprintf("; last sync %s at %s", status, lastUpdated)
	}

	out := awx.ObjectResult(obj, summary)
	out["status"] = status
	out["last_updated"] = lastUpdated
	return out, nil
}
