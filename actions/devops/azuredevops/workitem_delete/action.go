package devops_azuredevops_workitem_delete

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
	Name         = "Azure DevOps: Delete Work Item"
	Description  = "Delete a work item. By default it goes to the project's recycle bin and can be restored; turn on Destroy Permanently to erase it outright, which cannot be undone."
	Website      = "https://www.flomation.co"
	Icon         = "azure+trash"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID — optional; work items are addressable organisation-wide"},
	{Name: "work_item_id", Type: core.ConnectionTypeInteger, Label: "Work Item", Placeholder: "the work item ID, e.g. 42", Required: true},
	{Name: "destroy", Type: core.ConnectionTypeBoolean, Label: "Destroy Permanently", Placeholder: "erase instead of sending to the recycle bin — this cannot be undone"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Work Item ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Work Item"},
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

	q := url.Values{}
	destroy := azuredevops.OptionalBool("destroy", inputs)
	if destroy {
		q.Set("destroy", "true")
	}

	path := "/_apis/wit/workitems/" + strconv.Itoa(itemID)
	if project := azuredevops.OptionalString("project", inputs); project != "" {
		path = azuredevops.ProjectPath(project) + path
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{Method: http.MethodDelete, Path: path, Query: q})
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if err := azuredevops.CheckResponse(resp); err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	obj, _ := azuredevops.Decode(resp)

	summary := fmt.Sprintf("Deleted work item %d to the recycle bin", itemID)
	if destroy {
		summary = fmt.Sprintf("Permanently destroyed work item %d", itemID)
	}
	out := azuredevops.SuccessResult(strconv.Itoa(itemID), obj, summary)
	return out, nil
}
