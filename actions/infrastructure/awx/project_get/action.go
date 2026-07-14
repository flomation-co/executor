// Package infrastructure_awx_project_get fetches one AWX project.
package infrastructure_awx_project_get

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Project"
	Description  = "Fetch one project — its source-control settings, last sync status and the playbooks it contains."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+eye"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// --- AWX credential block (identical in all 59 AWX actions) ---
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

	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "Choose the project, or enter its AWX ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Project ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Last Sync Status"},
	{Name: "scm_revision", Type: core.ConnectionTypeString, Label: "Source Control Revision"},
	{Name: "last_updated", Type: core.ConnectionTypeString, Label: "Last Synced"},
	{Name: "playbook_files", Type: core.ConnectionTypeObject, Label: "Playbooks"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Project"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err // the ONE hard failure: the node is mis-configured
	}

	ctx, cancel := awx.Context()
	defer cancel()

	projectID, err := awx.RequiredInt("project_id", "Project", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	project, err := awx.GetResource(ctx, auth, fmt.Sprintf("projects/%d/", projectID), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	// The playbook list is a separate endpoint (a plain JSON ARRAY of filenames,
	// not a paginated envelope). It is best-effort: a project that has never been
	// synced has no checkout on disk and AWX answers 404 there, which must not
	// turn a perfectly good "get the project" into a failure.
	playbooks := fetchPlaybooks(ctx, auth, projectID)

	out := awx.ObjectResult(project, fmt.Sprintf("Fetched project %q (%d) — last sync %q, %d playbook(s)",
		awx.StringField(project, "name"), projectID, awx.StringField(project, "status"), len(playbooks)))
	out["status"] = awx.StringField(project, "status")
	out["scm_revision"] = awx.StringField(project, "scm_revision")
	out["last_updated"] = awx.StringField(project, "last_updated")
	out["playbook_files"] = playbooks
	return out, nil
}

// fetchPlaybooks reads GET projects/{id}/playbooks/, which returns a bare JSON
// array (["site.yml", …]) rather than AWX's usual paginated envelope. Any failure
// yields an empty list rather than an error.
func fetchPlaybooks(ctx context.Context, auth awx.Auth, projectID int64) []interface{} {
	resp, err := awx.Do(ctx, auth, http.MethodGet, fmt.Sprintf("projects/%d/playbooks/", projectID), nil)
	if err != nil || awx.CheckResponse(auth, resp) != nil {
		return []interface{}{}
	}
	var files []interface{}
	if err := json.Unmarshal(resp.Body, &files); err != nil || files == nil {
		return []interface{}{}
	}
	return files
}
