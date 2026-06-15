package github_create_issue

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Create Issue"
	Description  = "Create a new issue in a GitHub repository"
	Website      = "https://www.flomation.co"
	Icon         = "github+plus"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Body"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated label names"},
	{Name: "assignees", Type: core.ConnectionTypeString, Label: "Assignees", Placeholder: "Comma-separated usernames"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "issue_number", Type: core.ConnectionTypeString, Label: "Issue Number"},
	{Name: "html_url", Type: core.ConnectionTypeString, Label: "HTML URL"},
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
	title, err := github.RequiredString("title", inputs)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"title": title,
	}
	if v := github.OptionalString("body", inputs); v != "" {
		payload["body"] = v
	}
	if v := github.OptionalString("labels", inputs); v != "" {
		payload["labels"] = splitTrimmed(v)
	}
	if v := github.OptionalString("assignees", inputs); v != "" {
		payload["assignees"] = splitTrimmed(v)
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "POST", "/issues", payload)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to create issue: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var issue struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(resp.Body, &issue); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Created issue #%d: %s — %s", issue.Number, title, issue.HTMLURL),
		"issue_number": fmt.Sprintf("%d", issue.Number),
		"html_url":     issue.HTMLURL,
		"success":      true,
		"error":        "",
	}, nil
}

func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}
