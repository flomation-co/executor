// Package infrastructure_awx_host_get fetches one host.
//
// The Inventory input is NOT sent to AWX — a host id is globally unique, so the
// lookup needs nothing else. It exists purely so the editor can scope the Host
// live dropdown below it to one inventory; leaving it blank still works if the
// host id is supplied directly (from a variable, or from an upstream List Hosts).
package infrastructure_awx_host_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Host"
	Description  = "Fetch one host — its variables, whether it is enabled, and how its last job went."
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

	{Name: "inventory_id", Type: core.ConnectionTypeString, Label: "Inventory", Placeholder: "Only used to populate the Host list below"},
	{Name: "host_id", Type: core.ConnectionTypeString, Label: "Host", Placeholder: "The host to fetch", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Host ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Host"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled"},
	{Name: "has_active_failures", Type: core.ConnectionTypeBoolean, Label: "Has Active Failures"},
	{Name: "last_job", Type: core.ConnectionTypeObject, Label: "Last Job"},
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

	ctx, cancel := awx.Context()
	defer cancel()

	host, err := awx.GetResource(ctx, auth, fmt.Sprintf("hosts/%d/", hostID), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	name := awx.StringField(host, "name")
	out := awx.ObjectResult(host, fmt.Sprintf("Fetched host %s", name))
	out["name"] = name
	out["enabled"] = awx.BoolField(host, "enabled")
	out["has_active_failures"] = awx.BoolField(host, "has_active_failures")
	// summary_fields.last_job is where AWX puts the last job's id/name/status; the
	// top-level last_job is a bare id. Prefer the rich one when it is there.
	out["last_job"] = lastJob(host)
	return out, nil
}

// lastJob prefers summary_fields.last_job (id, name, status, finished) over the
// bare integer AWX puts at the top level, falling back to an empty object so the
// downstream output is always a map rather than sometimes a number and sometimes
// null.
func lastJob(host map[string]interface{}) interface{} {
	if summary, ok := host["summary_fields"].(map[string]interface{}); ok {
		if job, ok := summary["last_job"].(map[string]interface{}); ok {
			return job
		}
	}
	if id := awx.IDString(host["last_job"]); id != "" {
		return map[string]interface{}{"id": id}
	}
	return map[string]interface{}{}
}
