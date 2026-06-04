package gitlab_create_issue

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Create Issue"
	Description  = "Create a new issue in a GitLab project"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+plus"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Markdown description (optional)"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated label names"},
	{Name: "assignee_ids", Type: core.ConnectionTypeString, Label: "Assignee IDs", Placeholder: "Comma-separated user IDs"},
	{Name: "milestone_id", Type: core.ConnectionTypeString, Label: "Milestone ID"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "issue_iid", Type: core.ConnectionTypeString, Label: "Issue IID"},
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
	title, err := gitlab.RequiredString("title", inputs)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"title": title,
	}
	if v := gitlab.OptionalString("description", inputs); v != "" {
		body["description"] = v
	}
	if v := gitlab.OptionalString("labels", inputs); v != "" {
		body["labels"] = v
	}
	if v := gitlab.OptionalString("milestone_id", inputs); v != "" {
		body["milestone_id"] = v
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "POST", "/issues", body)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to create issue: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var issue struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
	}
	if err := json.Unmarshal(resp.Body, &issue); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created issue #%d: %s — %s", issue.IID, title, issue.WebURL),
		"issue_iid":   fmt.Sprintf("%d", issue.IID),
		"web_url":     issue.WebURL,
		"success":     true,
		"error":       "",
	}, nil
}
