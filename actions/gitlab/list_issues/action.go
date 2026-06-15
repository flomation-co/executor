package gitlab_list_issues

import (
	"encoding/json"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab List Issues"
	Description  = "List issues in a GitLab project with optional filters"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+list"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State", Options: []core.ConnectionOption{
		{Name: "All", Value: "all"},
		{Name: "Opened", Value: "opened"},
		{Name: "Closed", Value: "closed"},
	}},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Search in title and description"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated label names"},
	{Name: "assignee_username", Type: core.ConnectionTypeString, Label: "Assignee Username"},
	{Name: "per_page", Type: core.ConnectionTypeString, Label: "Per Page", Placeholder: "20 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "issues", Type: core.ConnectionTypeObject, Label: "Issues (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	params := url.Values{}
	if v := gitlab.OptionalString("state", inputs); v != "" {
		params.Set("state", v)
	}
	if v := gitlab.OptionalString("search", inputs); v != "" {
		params.Set("search", v)
	}
	if v := gitlab.OptionalString("labels", inputs); v != "" {
		params.Set("labels", v)
	}
	if v := gitlab.OptionalString("assignee_username", inputs); v != "" {
		params.Set("assignee_username", v)
	}
	if v := gitlab.OptionalString("per_page", inputs); v != "" {
		params.Set("per_page", v)
	}

	path := "/issues"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", path, nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to list issues: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var issues []interface{}
	if err := json.Unmarshal(resp.Body, &issues); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	summary := fmt.Sprintf("Found %d issue(s):\n", len(issues))
	for _, iss := range issues {
		if im, ok := iss.(map[string]interface{}); ok {
			summary += fmt.Sprintf("- #%v: %v [%v] — %v\n", im["iid"], im["title"], im["state"], im["web_url"])
		}
	}

	return map[string]interface{}{
		"tool_result": summary,
		"issues":      issues,
		"count":       int64(len(issues)),
		"success":     true,
		"error":       "",
	}, nil
}
