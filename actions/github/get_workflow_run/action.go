package github_get_workflow_run

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Get Workflow Run"
	Description  = "Retrieve details of a specific GitHub Actions workflow run"
	Website      = "https://www.flomation.co"
	Icon         = "github+play"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "run_id", Type: core.ConnectionTypeString, Label: "Run ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "conclusion", Type: core.ConnectionTypeString, Label: "Conclusion"},
	{Name: "head_branch", Type: core.ConnectionTypeString, Label: "Branch"},
	{Name: "html_url", Type: core.ConnectionTypeString, Label: "HTML URL"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Created At"},
	{Name: "data", Type: core.ConnectionTypeObject, Label: "Full Response (JSON)"},
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
	runID, err := github.RequiredString("run_id", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "GET", fmt.Sprintf("/actions/runs/%s", runID), nil)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to get workflow run: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var run struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HeadBranch string `json:"head_branch"`
		HTMLURL    string `json:"html_url"`
		CreatedAt  string `json:"created_at"`
	}
	if err := json.Unmarshal(resp.Body, &run); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	var fullData interface{}
	_ = json.Unmarshal(resp.Body, &fullData)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Run %s: %s/%s on %s — %s", runID, run.Status, run.Conclusion, run.HeadBranch, run.HTMLURL),
		"status":      run.Status,
		"conclusion":  run.Conclusion,
		"head_branch": run.HeadBranch,
		"html_url":    run.HTMLURL,
		"created_at":  run.CreatedAt,
		"data":        fullData,
		"success":     true,
		"error":       "",
	}, nil
}
