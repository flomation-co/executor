package github_get_issue

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Get Issue"
	Description  = "Retrieve details of a GitHub issue by number"
	Website      = "https://www.flomation.co"
	Icon         = "github+eye"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "issue_number", Type: core.ConnectionTypeString, Label: "Issue Number", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Body"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "user", Type: core.ConnectionTypeString, Label: "Author"},
	{Name: "html_url", Type: core.ConnectionTypeString, Label: "HTML URL"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels (JSON)"},
	{Name: "data", Type: core.ConnectionTypeObject, Label: "Full Response (JSON)"},
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

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "GET", fmt.Sprintf("/issues/%s", number), nil)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to get issue: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var issue struct {
		Title   string `json:"title"`
		Body    string `json:"body"`
		State   string `json:"state"`
		HTMLURL string `json:"html_url"`
		User    struct {
			Login string `json:"login"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(resp.Body, &issue); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	labelNames := make([]string, len(issue.Labels))
	for i, l := range issue.Labels {
		labelNames[i] = l.Name
	}
	labelsJSON, _ := json.Marshal(labelNames)

	var fullData interface{}
	_ = json.Unmarshal(resp.Body, &fullData)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Issue #%s: %s [%s] — %s", number, issue.Title, issue.State, issue.HTMLURL),
		"title":       issue.Title,
		"body":        issue.Body,
		"state":       issue.State,
		"user":        issue.User.Login,
		"html_url":    issue.HTMLURL,
		"labels":      string(labelsJSON),
		"data":        fullData,
		"success":     true,
		"error":       "",
	}, nil
}
