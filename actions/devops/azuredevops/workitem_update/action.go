package devops_azuredevops_workitem_update

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Update Work Item"
	Description  = "Update a work item's fields. Give an ordinary name/value map — \"state\", \"assigned to\", \"priority\", or full reference names; set a value to null to clear that field. The JSON-Patch document Azure DevOps requires is built for you."
	Website      = "https://www.flomation.co"
	Icon         = "azure+pen"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID — optional; work items are addressable organisation-wide"},
	{Name: "work_item_id", Type: core.ConnectionTypeInteger, Label: "Work Item", Placeholder: "the work item ID, e.g. 42", Required: true},
	{Name: "fields", Type: core.ConnectionTypeObject, Label: "Fields", Placeholder: "{\"state\": \"Active\", \"assigned to\": \"jane@contoso.com\"} — null clears a field", Required: true},
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
	itemID, err := azuredevops.RequiredInt("work_item_id", "Work Item", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	fields, err := azuredevops.ObjectInput("fields", "Fields", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if len(fields) == 0 {
		return azuredevops.ErrorResult(`Fields is required — supply at least one field, e.g. {"state": "Active"}`), nil
	}
	ops, err := azuredevops.FieldPatch(fields)
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

	path := "/_apis/wit/workitems/" + strconv.Itoa(itemID)
	if project := azuredevops.OptionalString("project", inputs); project != "" {
		path = azuredevops.ProjectPath(project) + path
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method:      http.MethodPatch,
		Path:        path,
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
	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Updated work item %d (%d field(s))", itemID, len(ops)))
	out["url"], _ = obj["url"].(string)
	return out, nil
}
