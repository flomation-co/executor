package github_list_pull_request_reviews

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub List PR Reviews"
	Description  = "List reviews on a GitHub pull request"
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
	{Name: "pull_number", Type: core.ConnectionTypeString, Label: "Pull Request Number", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "reviews", Type: core.ConnectionTypeObject, Label: "Reviews (JSON)"},
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
	number, err := github.RequiredString("pull_number", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "GET", fmt.Sprintf("/pulls/%s/reviews", number), nil)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to list reviews: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var reviews []interface{}
	if err := json.Unmarshal(resp.Body, &reviews); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("PR #%s has %d review(s)", number, len(reviews)),
		"reviews":     reviews,
		"count":       int64(len(reviews)),
		"success":     true,
		"error":       "",
	}, nil
}
