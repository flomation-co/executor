package github_merge_pull_request

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Merge Pull Request"
	Description  = "Merge a GitHub pull request"
	Website      = "https://www.flomation.co"
	Icon         = "github+check"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "pull_number", Type: core.ConnectionTypeString, Label: "Pull Request Number", Required: true},
	{Name: "merge_method", Type: core.ConnectionTypeString, Label: "Merge Method", Options: []core.ConnectionOption{
		{Name: "Merge Commit", Value: "merge"},
		{Name: "Squash", Value: "squash"},
		{Name: "Rebase", Value: "rebase"},
	}},
	{Name: "commit_title", Type: core.ConnectionTypeString, Label: "Commit Title"},
	{Name: "commit_message", Type: core.ConnectionTypeText, Label: "Commit Message"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "sha", Type: core.ConnectionTypeString, Label: "Merge Commit SHA"},
	{Name: "merged", Type: core.ConnectionTypeBoolean, Label: "Merged"},
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

	payload := map[string]interface{}{}
	if v := github.OptionalString("merge_method", inputs); v != "" {
		payload["merge_method"] = v
	}
	if v := github.OptionalString("commit_title", inputs); v != "" {
		payload["commit_title"] = v
	}
	if v := github.OptionalString("commit_message", inputs); v != "" {
		payload["commit_message"] = v
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "PUT", fmt.Sprintf("/pulls/%s/merge", number), payload)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to merge: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var result struct {
		SHA     string `json:"sha"`
		Merged  bool   `json:"merged"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Merged PR #%s: %s (sha: %s)", number, result.Message, result.SHA),
		"sha":         result.SHA,
		"merged":      result.Merged,
		"success":     true,
		"error":       "",
	}, nil
}
