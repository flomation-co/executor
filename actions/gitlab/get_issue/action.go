package gitlab_get_issue

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Get Issue"
	Description  = "Retrieve details of a GitLab issue by IID"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+eye"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "issue_iid", Type: core.ConnectionTypeString, Label: "Issue IID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "author", Type: core.ConnectionTypeString, Label: "Author"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels (JSON)"},
	{Name: "data", Type: core.ConnectionTypeObject, Label: "Full Response (JSON)"},
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
	iid, err := gitlab.RequiredString("issue_iid", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", fmt.Sprintf("/issues/%s", iid), nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to get issue: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var issue struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		State       string   `json:"state"`
		WebURL      string   `json:"web_url"`
		Labels      []string `json:"labels"`
		Author      struct {
			Username string `json:"username"`
		} `json:"author"`
	}
	if err := json.Unmarshal(resp.Body, &issue); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	labelsJSON, _ := json.Marshal(issue.Labels)
	var fullData interface{}
	_ = json.Unmarshal(resp.Body, &fullData)

	summary := fmt.Sprintf("Issue #%s: %s\nState: %s | Author: @%s\nURL: %s\nLabels: %s\n",
		iid, issue.Title, issue.State, issue.Author.Username, issue.WebURL, string(labelsJSON))
	if issue.Description != "" {
		summary += fmt.Sprintf("\nDescription:\n%s\n", issue.Description)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"title":       issue.Title,
		"description": issue.Description,
		"state":       issue.State,
		"author":      issue.Author.Username,
		"web_url":     issue.WebURL,
		"labels":      string(labelsJSON),
		"data":        fullData,
		"success":     true,
		"error":       "",
	}, nil
}
