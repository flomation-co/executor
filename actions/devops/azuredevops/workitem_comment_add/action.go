package devops_azuredevops_workitem_comment_add

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Add Work Item Comment"
	Description  = "Add a comment to a work item. Unlike creating or updating one, a comment is plain text on its own endpoint — no field map involved. Comments accept simple HTML for formatting."
	Website      = "https://www.flomation.co"
	Icon         = "azure+comment"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "work_item_id", Type: core.ConnectionTypeInteger, Label: "Work Item", Placeholder: "the work item ID, e.g. 42", Required: true},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Comment", Placeholder: "what you want to say — simple HTML is allowed", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Comment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Comment"},
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
	itemID, err := azuredevops.RequiredInt("work_item_id", "Work Item", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	text, err := azuredevops.RequiredString("text", "Comment", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	// Comments are still preview-only in 7.1 and reject the GA version string,
	// so this call pins its own — see azuredevops.CommentsAPIVersion.
	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method:     http.MethodPost,
		Path:       fmt.Sprintf("%s/_apis/wit/workItems/%d/comments", azuredevops.ProjectPath(project), itemID),
		Body:       map[string]interface{}{"text": text},
		APIVersion: azuredevops.CommentsAPIVersion,
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
	return azuredevops.ResourceResult(obj, fmt.Sprintf("Commented on work item %d", itemID)), nil
}
