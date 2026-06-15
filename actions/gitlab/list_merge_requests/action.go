package gitlab_list_merge_requests

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
	Name         = "GitLab List Merge Requests"
	Description  = "List merge requests in a GitLab project with optional filters"
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
		{Name: "Merged", Value: "merged"},
	}},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search", Placeholder: "Search in title and description"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated label names"},
	{Name: "author_username", Type: core.ConnectionTypeString, Label: "Author Username"},
	{Name: "per_page", Type: core.ConnectionTypeString, Label: "Per Page", Placeholder: "20 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "merge_requests", Type: core.ConnectionTypeObject, Label: "Merge Requests (JSON)"},
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
	if v := gitlab.OptionalString("author_username", inputs); v != "" {
		params.Set("author_username", v)
	}
	if v := gitlab.OptionalString("per_page", inputs); v != "" {
		params.Set("per_page", v)
	}

	path := "/merge_requests"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", path, nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to list merge requests: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var mrs []interface{}
	if err := json.Unmarshal(resp.Body, &mrs); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	summary := fmt.Sprintf("Found %d merge request(s):\n", len(mrs))
	for _, mr := range mrs {
		if m, ok := mr.(map[string]interface{}); ok {
			summary += fmt.Sprintf("- !%v: %v [%v] — %v\n", m["iid"], m["title"], m["state"], m["web_url"])
		}
	}

	return map[string]interface{}{
		"tool_result":    summary,
		"merge_requests": mrs,
		"count":          int64(len(mrs)),
		"success":        true,
		"error":          "",
	}, nil
}
