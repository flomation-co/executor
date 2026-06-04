package gitlab_update_merge_request

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Update Merge Request"
	Description  = "Update an existing merge request in a GitLab project"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+pencil"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "merge_request_iid", Type: core.ConnectionTypeString, Label: "Merge Request IID", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated label names"},
	{Name: "assignee_ids", Type: core.ConnectionTypeString, Label: "Assignee IDs", Placeholder: "Comma-separated user IDs"},
	{Name: "reviewer_ids", Type: core.ConnectionTypeString, Label: "Reviewer IDs", Placeholder: "Comma-separated user IDs"},
	{Name: "state_event", Type: core.ConnectionTypeString, Label: "State Event", Options: []core.ConnectionOption{
		{Name: "No Change", Value: ""},
		{Name: "Close", Value: "close"},
		{Name: "Reopen", Value: "reopen"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := gitlab.GetAccessToken(inputs)
	if err != nil {
		return nil, err
	}
	baseURL := gitlab.GetBaseURL(inputs)
	projectID, err := gitlab.GetProjectID(inputs)
	if err != nil {
		return nil, err
	}
	iid, err := gitlab.RequiredString("merge_request_iid", inputs)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{}
	if v := gitlab.OptionalString("title", inputs); v != "" {
		body["title"] = v
	}
	if v := gitlab.OptionalString("description", inputs); v != "" {
		body["description"] = v
	}
	if v := gitlab.OptionalString("labels", inputs); v != "" {
		body["labels"] = v
	}
	if v := gitlab.OptionalString("state_event", inputs); v != "" {
		body["state_event"] = v
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "PUT", fmt.Sprintf("/merge_requests/%s", iid), body)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to update merge request: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var mr struct {
		WebURL string `json:"web_url"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(resp.Body, &mr); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated MR !%s: %s — %s", iid, mr.Title, mr.WebURL),
		"web_url":     mr.WebURL,
		"success":     true,
		"error":       "",
	}, nil
}
