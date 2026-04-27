package github_create_pull_request_review

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Create PR Review"
	Description  = "Submit a review on a GitHub pull request (approve, request changes, or comment)"
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
	{Name: "event", Type: core.ConnectionTypeString, Label: "Review Event", Required: true, Options: []core.ConnectionOption{
		{Name: "Approve", Value: "APPROVE"},
		{Name: "Request Changes", Value: "REQUEST_CHANGES"},
		{Name: "Comment", Value: "COMMENT"},
	}},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Review Body", Placeholder: "Review comment (required for REQUEST_CHANGES)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "review_id", Type: core.ConnectionTypeString, Label: "Review ID"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "Review State"},
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
	event, err := github.RequiredString("event", inputs)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"event": event,
	}
	if v := github.OptionalString("body", inputs); v != "" {
		payload["body"] = v
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "POST", fmt.Sprintf("/pulls/%s/reviews", number), payload)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to create review: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var review struct {
		ID    int    `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(resp.Body, &review); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Submitted %s review %d on PR #%s", review.State, review.ID, number),
		"review_id":   fmt.Sprintf("%d", review.ID),
		"state":       review.State,
		"success":     true,
		"error":       "",
	}, nil
}
