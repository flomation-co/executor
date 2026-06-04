package github_dismiss_pull_request_review

import (
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Dismiss PR Review"
	Description  = "Dismiss a review on a GitHub pull request"
	Website      = "https://www.flomation.co"
	Icon         = "github+xmark"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "pull_number", Type: core.ConnectionTypeString, Label: "Pull Request Number", Required: true},
	{Name: "review_id", Type: core.ConnectionTypeString, Label: "Review ID", Required: true},
	{Name: "message", Type: core.ConnectionTypeText, Label: "Dismissal Message", Required: true},
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
	number, err := github.RequiredString("pull_number", inputs)
	if err != nil {
		return nil, err
	}
	reviewID, err := github.RequiredString("review_id", inputs)
	if err != nil {
		return nil, err
	}
	message, err := github.RequiredString("message", inputs)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"message": message,
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "PUT", fmt.Sprintf("/pulls/%s/reviews/%s/dismissals", number, reviewID), payload)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to dismiss review: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Dismissed review %s on PR #%s", reviewID, number),
		"success":     true,
		"error":       "",
	}, nil
}
