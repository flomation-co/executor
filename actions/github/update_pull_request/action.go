package github_update_pull_request

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Update Pull Request"
	Description  = "Update an existing GitHub pull request"
	Website      = "https://www.flomation.co"
	Icon         = "github+pencil"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "pull_number", Type: core.ConnectionTypeString, Label: "Pull Request Number", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Description"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State", Options: []core.ConnectionOption{
		{Name: "No Change", Value: ""},
		{Name: "Open", Value: "open"},
		{Name: "Closed", Value: "closed"},
	}},
	{Name: "base", Type: core.ConnectionTypeString, Label: "Base Branch"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	number, err := github.RequiredString("pull_number", inputs)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{}
	if v := github.OptionalString("title", inputs); v != "" {
		payload["title"] = v
	}
	if v := github.OptionalString("body", inputs); v != "" {
		payload["body"] = v
	}
	if v := github.OptionalString("state", inputs); v != "" {
		payload["state"] = v
	}
	if v := github.OptionalString("base", inputs); v != "" {
		payload["base"] = v
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "PATCH", fmt.Sprintf("/pulls/%s", number), payload)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to update pull request: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var pr struct {
		HTMLURL string `json:"html_url"`
		Title   string `json:"title"`
	}
	if err := json.Unmarshal(resp.Body, &pr); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated PR #%s: %s — %s", number, pr.Title, pr.HTMLURL),
		"html_url":    pr.HTMLURL,
		"success":     true,
		"error":       "",
	}, nil
}
