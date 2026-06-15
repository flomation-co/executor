package github_add_issue_comment

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Add Issue Comment"
	Description  = "Add a comment to a GitHub issue"
	Website      = "https://www.flomation.co"
	Icon         = "github+comments"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "issue_number", Type: core.ConnectionTypeString, Label: "Issue Number", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Comment Body", Placeholder: "Markdown comment text", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID"},
	{Name: "html_url", Type: core.ConnectionTypeString, Label: "Comment URL"},
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
	number, err := github.RequiredString("issue_number", inputs)
	if err != nil {
		return nil, err
	}
	commentBody, err := github.RequiredString("body", inputs)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"body": commentBody,
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "POST", fmt.Sprintf("/issues/%s/comments", number), payload)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to add comment: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var comment struct {
		ID      int    `json:"id"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(resp.Body, &comment); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Added comment %d to issue #%s", comment.ID, number),
		"comment_id":  fmt.Sprintf("%d", comment.ID),
		"html_url":    comment.HTMLURL,
		"success":     true,
		"error":       "",
	}, nil
}
