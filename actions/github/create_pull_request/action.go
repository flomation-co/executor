package github_create_pull_request

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Create Pull Request"
	Description  = "Create a new pull request in a GitHub repository"
	Website      = "https://www.flomation.co"
	Icon         = "github+plus"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Required: true},
	{Name: "head", Type: core.ConnectionTypeString, Label: "Head Branch", Required: true},
	{Name: "base", Type: core.ConnectionTypeString, Label: "Base Branch", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Description"},
	{Name: "draft", Type: core.ConnectionTypeBoolean, Label: "Draft PR"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pull_number", Type: core.ConnectionTypeString, Label: "Pull Request Number"},
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
	head, err := github.RequiredString("head", inputs)
	if err != nil {
		return nil, err
	}
	base, err := github.RequiredString("base", inputs)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"title": title,
		"head":  head,
		"base":  base,
	}
	if v := github.OptionalString("body", inputs); v != "" {
		payload["body"] = v
	}
	if v := github.OptionalBool("draft", inputs); v != nil {
		payload["draft"] = *v
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "POST", "/pulls", payload)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to create pull request: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var pr struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(resp.Body, &pr); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created PR #%d: %s — %s", pr.Number, title, pr.HTMLURL),
		"pull_number": fmt.Sprintf("%d", pr.Number),
		"html_url":    pr.HTMLURL,
		"success":     true,
		"error":       "",
	}, nil
}
