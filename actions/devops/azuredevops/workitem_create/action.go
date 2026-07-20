package devops_azuredevops_workitem_create

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Create Work Item"
	Description  = "Create a work item (Bug, Task, User Story, …). Fields are given as an ordinary name/value map — \"title\", \"assigned to\", \"priority\" and the like, or full reference names such as System.AreaPath. The JSON-Patch document Azure DevOps actually requires is built for you."
	Website      = "https://www.flomation.co"
	Icon         = "azure+plus"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "work_item_type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "Bug, Task, User Story — see List Work Item Types (process templates differ)", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "a short summary of the work", Required: true},
	{Name: "fields", Type: core.ConnectionTypeObject, Label: "Fields", Placeholder: "{\"description\": \"…\", \"assigned to\": \"jane@contoso.com\", \"priority\": 2} — shorthands or reference names"},
	{Name: "bypass_rules", Type: core.ConnectionTypeBoolean, Label: "Bypass Rules", Placeholder: "skip the process template's validation rules (needs elevated permission)"},
	{Name: "suppress_notifications", Type: core.ConnectionTypeBoolean, Label: "Suppress Notifications", Placeholder: "do not email anyone about this change"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Work Item ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Work Item"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Work Item URL"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := azuredevops.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	project, err := azuredevops.RequiredString("project", "Project", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	itemType, err := azuredevops.RequiredString("work_item_type", "Type", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	title, err := azuredevops.RequiredString("title", "Title", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	fields, err := azuredevops.ObjectInput("fields", "Fields", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	// Title is a first-class input because every work item needs one, but the
	// field map still wins if it names the title too — same power-user-last-word
	// precedence the other nodes use for additional_fields.
	merged := map[string]interface{}{"System.Title": title}
	for k, v := range fields {
		ref, err := azuredevops.ResolveFieldName(k)
		if err == nil && ref == "System.Title" {
			delete(merged, "System.Title")
		}
		merged[k] = v
	}
	ops, err := azuredevops.FieldPatch(merged)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	patch, err := azuredevops.EncodePatch(ops)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	if azuredevops.OptionalBool("bypass_rules", inputs) {
		q.Set("bypassRules", "true")
	}
	if azuredevops.OptionalBool("suppress_notifications", inputs) {
		q.Set("suppressNotifications", "true")
	}

	// The type goes in the path behind a LITERAL $ — /workitems/$Bug. The dollar
	// is a legal path sub-delimiter and must NOT be percent-encoded, so only the
	// type name itself is escaped.
	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method:      http.MethodPost,
		Path:        azuredevops.ProjectPath(project) + "/_apis/wit/workitems/$" + url.PathEscape(itemType),
		Query:       q,
		RawBody:     patch,
		ContentType: azuredevops.JSONPatchContentType,
	})
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if err := azuredevops.CheckResponse(resp); err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	obj, err := azuredevops.Decode(resp)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Created %s %s: %s", itemType, azuredevops.IDOf(obj), title))
	out["url"], _ = obj["url"].(string)
	return out, nil
}
