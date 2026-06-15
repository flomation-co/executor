package github_list_pull_requests

import (
	"encoding/json"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	github "flomation.app/automate/executor/actions/github"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitHub List Pull Requests"
	Description  = "List pull requests in a GitHub repository with optional filters"
	Website      = "https://www.flomation.co"
	Icon         = "github+list"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitHub Access Token", Placeholder: "ghp_...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitHub API Base URL", Placeholder: "https://api.github.com"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Repository Owner", Required: true},
	{Name: "repo", Type: core.ConnectionTypeString, Label: "Repository Name", Required: true},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State", Options: []core.ConnectionOption{
		{Name: "Open", Value: "open"},
		{Name: "Closed", Value: "closed"},
		{Name: "All", Value: "all"},
	}},
	{Name: "head", Type: core.ConnectionTypeString, Label: "Head Branch", Placeholder: "user:branch"},
	{Name: "base", Type: core.ConnectionTypeString, Label: "Base Branch"},
	{Name: "sort", Type: core.ConnectionTypeString, Label: "Sort By", Options: []core.ConnectionOption{
		{Name: "Created", Value: "created"},
		{Name: "Updated", Value: "updated"},
		{Name: "Popularity", Value: "popularity"},
	}},
	{Name: "per_page", Type: core.ConnectionTypeString, Label: "Per Page", Placeholder: "30 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pull_requests", Type: core.ConnectionTypeObject, Label: "Pull Requests (JSON)"},
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

	params := url.Values{}
	if v := github.OptionalString("state", inputs); v != "" {
		params.Set("state", v)
	}
	if v := github.OptionalString("head", inputs); v != "" {
		params.Set("head", v)
	}
	if v := github.OptionalString("base", inputs); v != "" {
		params.Set("base", v)
	}
	if v := github.OptionalString("sort", inputs); v != "" {
		params.Set("sort", v)
	}
	if v := github.OptionalString("per_page", inputs); v != "" {
		params.Set("per_page", v)
	}

	path := "/pulls"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := github.RepoAPI(token, baseURL, owner, repo, "GET", path, nil)
	if err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to list pull requests: %s", err)), nil
	}
	if err := github.CheckResponse(resp); err != nil {
		return github.ErrorResult(err.Error()), nil
	}

	var prs []interface{}
	if err := json.Unmarshal(resp.Body, &prs); err != nil {
		return github.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Found %d pull request(s)", len(prs)),
		"pull_requests": prs,
		"count":         int64(len(prs)),
		"success":       true,
		"error":         "",
	}, nil
}
