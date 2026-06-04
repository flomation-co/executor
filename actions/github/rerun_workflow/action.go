package github_rerun_workflow

import (
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Re-run Workflow"
	Description  = "Re-run a GitHub Actions workflow run"
	Website      = "https://www.flomation.co"
	Icon         = "github+refresh"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "run_id", Type: core.ConnectionTypeString, Label: "Run ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "POST", fmt.Sprintf("/actions/runs/%s/rerun", runID), nil)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to re-run workflow: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Re-running workflow run %s", runID),
		"success":     true,
		"error":       "",
	}, nil
}
