package gitlab_create_merge_request

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Create Merge Request"
	Description  = "Create a new merge request in a GitLab project"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+plus"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "source_branch", Type: core.ConnectionTypeString, Label: "Source Branch", Required: true},
	{Name: "target_branch", Type: core.ConnectionTypeString, Label: "Target Branch", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Markdown description (optional)"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated label names"},
	{Name: "assignee_ids", Type: core.ConnectionTypeString, Label: "Assignee IDs", Placeholder: "Comma-separated user IDs"},
	{Name: "reviewer_ids", Type: core.ConnectionTypeString, Label: "Reviewer IDs", Placeholder: "Comma-separated user IDs"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "merge_request_iid", Type: core.ConnectionTypeString, Label: "Merge Request IID"},
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
	sourceBranch, err := gitlab.RequiredString("source_branch", inputs)
	if err != nil {
		return nil, err
	}
	targetBranch, err := gitlab.RequiredString("target_branch", inputs)
	if err != nil {
		return nil, err
	}
	title, err := gitlab.RequiredString("title", inputs)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"source_branch": sourceBranch,
		"target_branch": targetBranch,
		"title":         title,
	}

	if v := gitlab.OptionalString("description", inputs); v != "" {
		body["description"] = v
	}
	if v := gitlab.OptionalString("labels", inputs); v != "" {
		body["labels"] = v
	}
	if v := gitlab.OptionalString("assignee_ids", inputs); v != "" {
		body["assignee_ids"] = parseCSVInts(v)
	}
	if v := gitlab.OptionalString("reviewer_ids", inputs); v != "" {
		body["reviewer_ids"] = parseCSVInts(v)
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "POST", "/merge_requests", body)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to create merge request: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var mr struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
	}
	if err := json.Unmarshal(resp.Body, &mr); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Created MR !%d: %s — %s", mr.IID, title, mr.WebURL),
		"merge_request_iid": fmt.Sprintf("%d", mr.IID),
		"web_url":           mr.WebURL,
		"success":           true,
		"error":             "",
	}, nil
}

func parseCSVInts(s string) []int {
	var result []int
	var current string
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			v := 0
			for _, c := range current {
				if c >= '0' && c <= '9' {
					v = v*10 + int(c-'0')
				}
			}
			if current != "" {
				result = append(result, v)
			}
			current = ""
		} else if s[i] != ' ' {
			current += string(s[i])
		}
	}
	return result
}
