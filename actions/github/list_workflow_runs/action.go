package github_list_workflow_runs

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
	Name         = "GitHub List Workflow Runs"
	Description  = "List GitHub Actions workflow runs for a repository"
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
	{Name: "branch", Type: core.ConnectionTypeString, Label: "Branch"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{
		{Name: "All", Value: ""},
		{Name: "Completed", Value: "completed"},
		{Name: "In Progress", Value: "in_progress"},
		{Name: "Queued", Value: "queued"},
	}},
	{Name: "per_page", Type: core.ConnectionTypeString, Label: "Per Page", Placeholder: "30 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "workflow_runs", Type: core.ConnectionTypeObject, Label: "Workflow Runs (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Total Count"},
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
	if v := github.OptionalString("branch", inputs); v != "" {
		params.Set("branch", v)
	}
	if v := github.OptionalString("status", inputs); v != "" {
		params.Set("status", v)
	}
	if v := github.OptionalString("per_page", inputs); v != "" {
		params.Set("per_page", v)
	}

	path := "/actions/runs"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "GET", path, nil)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to list workflow runs: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var result struct {
		TotalCount   int           `json:"total_count"`
		WorkflowRuns []interface{} `json:"workflow_runs"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Found %d workflow run(s)", result.TotalCount),
		"workflow_runs": result.WorkflowRuns,
		"count":         int64(result.TotalCount),
		"success":       true,
		"error":         "",
	}, nil
}
