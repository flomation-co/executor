package github_trigger_workflow

import (
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Trigger Workflow"
	Description  = "Trigger a GitHub Actions workflow dispatch event"
	Website      = "https://www.flomation.co"
	Icon         = "github"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "workflow_id", Type: core.ConnectionTypeString, Label: "Workflow ID or Filename", Placeholder: "ID or filename (e.g. deploy.yml)", Required: true},
	{Name: "ref", Type: core.ConnectionTypeString, Label: "Ref", Placeholder: "Branch or tag", Required: true},
	{Name: "inputs", Type: core.ConnectionTypeKeyValueArray, Label: "Workflow Inputs"},
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
	workflowID, err := github.RequiredString("workflow_id", inputs)
	if err != nil {
		return nil, err
	}
	ref, err := github.RequiredString("ref", inputs)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"ref": ref,
	}

	conn := core.FindConnection("inputs", inputs)
	if conn != nil {
		pairs := conn.KeyValuePairs()
		if len(pairs) > 0 {
			workflowInputs := make(map[string]string, len(pairs))
			for _, p := range pairs {
				workflowInputs[p.Key] = p.Value
			}
			payload["inputs"] = workflowInputs
		}
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "POST", fmt.Sprintf("/actions/workflows/%s/dispatches", workflowID), payload)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to trigger workflow: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Triggered workflow %s on %s", workflowID, ref),
		"success":     true,
		"error":       "",
	}, nil
}
