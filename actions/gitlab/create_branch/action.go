package gitlab_create_branch

import (
	"encoding/json"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Create Branch"
	Description  = "Create a new branch in a GitLab project via the API (no git checkout)"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+code-branch"
	Date         = "29/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "branch", Type: core.ConnectionTypeString, Label: "New Branch Name", Placeholder: "feature/my-change", Required: true},
	{Name: "ref", Type: core.ConnectionTypeString, Label: "Source Ref", Placeholder: "main — branch, tag or commit SHA to branch from", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "branch", Type: core.ConnectionTypeString, Label: "Branch name"},
	{Name: "commit_sha", Type: core.ConnectionTypeString, Label: "Commit SHA"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := gitlab.GetAccessToken(inputs)
	if err != nil {
		return nil, err
	}
	baseURL := gitlab.GetBaseURL(inputs)
	projectID, err := gitlab.GetProjectID(inputs)
	if err != nil {
		return nil, err
	}
	branch, err := gitlab.RequiredString("branch", inputs)
	if err != nil {
		return nil, err
	}
	ref, err := gitlab.RequiredString("ref", inputs)
	if err != nil {
		return nil, err
	}

	// GitLab takes branch + ref as query params on POST /repository/branches.
	path := fmt.Sprintf("/repository/branches?branch=%s&ref=%s", url.QueryEscape(branch), url.QueryEscape(ref))
	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "POST", path, nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to create branch: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var out struct {
		Name   string `json:"name"`
		WebURL string `json:"web_url"`
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created branch %s from %s (%s)", out.Name, ref, out.Commit.ID),
		"branch":      out.Name,
		"commit_sha":  out.Commit.ID,
		"web_url":     out.WebURL,
		"success":     true,
		"error":       "",
	}, nil
}
