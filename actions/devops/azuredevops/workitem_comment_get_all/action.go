package devops_azuredevops_workitem_comment_get_all

import (
	"encoding/json"
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
	Name         = "Azure DevOps: List Work Item Comments"
	Description  = "List the comments on a work item, newest last. Note these are the discussion comments, not the field-change history."
	Website      = "https://www.flomation.co"
	Icon         = "azure+comments"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "work_item_id", Type: core.ConnectionTypeInteger, Label: "Work Item", Placeholder: "the work item ID, e.g. 42", Required: true},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "how many to return (default 50, max 1000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Comments"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	limit, set := azuredevops.OptionalInt("limit", inputs)
	q := url.Values{}
	q.Set("$top", strconv.Itoa(azuredevops.ClampLimit(limit, set)))

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method:     http.MethodGet,
		Path:       fmt.Sprintf("%s/_apis/wit/workItems/%d/comments", azuredevops.ProjectPath(project), itemID),
		Query:      q,
		APIVersion: azuredevops.CommentsAPIVersion,
	})
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if err := azuredevops.CheckResponse(resp); err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	// This endpoint breaks the {count, value:[…]} convention every other list
	// endpoint follows: the items are under "comments". DecodeList would return
	// an empty list here, silently.
	var env struct {
		TotalCount int           `json:"totalCount"`
		Comments   []interface{} `json:"comments"`
	}
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return azuredevops.ErrorResult("failed to parse the comments response: " + err.Error()), nil
	}
	return azuredevops.ListResult(env.Comments, fmt.Sprintf("Found %d comment(s) on work item %d", len(env.Comments), itemID)), nil
}
