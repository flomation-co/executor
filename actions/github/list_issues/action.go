package github_list_issues

import (
	"encoding/json"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub List Issues"
	Description  = "List issues in a GitHub repository with optional filters"
	Website      = "https://www.flomation.co"
	Icon         = "github+list"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State", Options: []core.ConnectionOption{
		{Name: "Open", Value: "open"},
		{Name: "Closed", Value: "closed"},
		{Name: "All", Value: "all"},
	}},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated label names"},
	{Name: "assignee", Type: core.ConnectionTypeString, Label: "Assignee", Placeholder: "Username or * for any"},
	{Name: "sort", Type: core.ConnectionTypeString, Label: "Sort By", Options: []core.ConnectionOption{
		{Name: "Created", Value: "created"},
		{Name: "Updated", Value: "updated"},
		{Name: "Comments", Value: "comments"},
	}},
	{Name: "per_page", Type: core.ConnectionTypeString, Label: "Per Page", Placeholder: "30 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "issues", Type: core.ConnectionTypeObject, Label: "Issues (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := github.GetAccessToken(inputs)
	if err != nil {
		return nil, err
	}
	baseURL := github.GetBaseURL(inputs)
	owner, err := github.GetOwner(inputs)
	if err != nil {
		return nil, err
	}
	repo, err := github.GetRepo(inputs)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	if v := github.OptionalString("state", inputs); v != "" {
		params.Set("state", v)
	}
	if v := github.OptionalString("labels", inputs); v != "" {
		params.Set("labels", v)
	}
	if v := github.OptionalString("assignee", inputs); v != "" {
		params.Set("assignee", v)
	}
	if v := github.OptionalString("sort", inputs); v != "" {
		params.Set("sort", v)
	}
	if v := github.OptionalString("per_page", inputs); v != "" {
		params.Set("per_page", v)
	}

	path := "/issues"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "GET", path, nil)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to list issues: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var issues []interface{}
	if err := json.Unmarshal(resp.Body, &issues); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d issue(s)", len(issues)),
		"issues":      issues,
		"count":       int64(len(issues)),
		"success":     true,
		"error":       "",
	}, nil
}
