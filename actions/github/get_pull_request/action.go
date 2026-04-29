package github_get_pull_request

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub Get Pull Request"
	Description  = "Retrieve details of a GitHub pull request by number"
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
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Body"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "user", Type: core.ConnectionTypeString, Label: "Author"},
	{Name: "head_ref", Type: core.ConnectionTypeString, Label: "Head Branch"},
	{Name: "base_ref", Type: core.ConnectionTypeString, Label: "Base Branch"},
	{Name: "html_url", Type: core.ConnectionTypeString, Label: "HTML URL"},
	{Name: "mergeable", Type: core.ConnectionTypeBoolean, Label: "Mergeable"},
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
	number, err := github.RequiredString("pull_number", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "GET", fmt.Sprintf("/pulls/%s", number), nil)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to get pull request: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var pr struct {
		Title     string `json:"title"`
		Body      string `json:"body"`
		State     string `json:"state"`
		HTMLURL   string `json:"html_url"`
		Mergeable *bool  `json:"mergeable"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(resp.Body, &pr); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	labelNames := make([]string, len(pr.Labels))
	for i, l := range pr.Labels {
		labelNames[i] = l.Name
	}
	labelsJSON, _ := json.Marshal(labelNames)

	var fullData interface{}
	_ = json.Unmarshal(resp.Body, &fullData)

	mergeable := false
	if pr.Mergeable != nil {
		mergeable = *pr.Mergeable
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("PR #%s: %s [%s] — %s", number, pr.Title, pr.State, pr.HTMLURL),
		"title":       pr.Title,
		"body":        pr.Body,
		"state":       pr.State,
		"user":        pr.User.Login,
		"head_ref":    pr.Head.Ref,
		"base_ref":    pr.Base.Ref,
		"html_url":    pr.HTMLURL,
		"mergeable":   mergeable,
		"labels":      string(labelsJSON),
		"data":        fullData,
		"success":     true,
		"error":       "",
	}, nil
}
