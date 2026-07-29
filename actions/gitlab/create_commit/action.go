package gitlab_create_commit

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Create Commit"
	Description  = "Commit one or more file changes to a branch in a single commit via the API (no git)"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+code"
	Date         = "29/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "branch", Type: core.ConnectionTypeString, Label: "Branch", Placeholder: "feature/my-change", Required: true},
	{Name: "commit_message", Type: core.ConnectionTypeString, Label: "Commit Message", Required: true},
	{Name: "actions", Type: core.ConnectionTypeCode, Label: "Actions (JSON array)", Placeholder: "[{\"action\":\"create\",\"file_path\":\"NOTES.md\",\"content\":\"# Notes\"}] — action is create|update|delete|move", Required: true},
	{Name: "start_branch", Type: core.ConnectionTypeString, Label: "Start Branch", Placeholder: "main — creates Branch from this ref if it doesn't exist yet"},
	{Name: "author_name", Type: core.ConnectionTypeString, Label: "Author Name", Placeholder: "Optional"},
	{Name: "author_email", Type: core.ConnectionTypeString, Label: "Author Email", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	commitMessage, err := gitlab.RequiredString("commit_message", inputs)
	if err != nil {
		return nil, err
	}
	actionsRaw, err := gitlab.RequiredString("actions", inputs)
	if err != nil {
		return nil, err
	}

	var actions []map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(actionsRaw)), &actions); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Actions must be a JSON array of {action,file_path,content}: %s", err)), nil
	}
	if len(actions) == 0 {
		return gitlab.ErrorResult("Actions is empty — provide at least one file change"), nil
	}

	body := map[string]interface{}{
		"branch":         branch,
		"commit_message": commitMessage,
		"actions":        actions,
	}
	if v := gitlab.OptionalString("start_branch", inputs); v != "" {
		body["start_branch"] = v
	}
	if v := gitlab.OptionalString("author_name", inputs); v != "" {
		body["author_name"] = v
	}
	if v := gitlab.OptionalString("author_email", inputs); v != "" {
		body["author_email"] = v
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "POST", "/repository/commits", body)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to create commit: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var out struct {
		ID     string `json:"id"`
		WebURL string `json:"web_url"`
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Committed %d change(s) to %s (%s)", len(actions), branch, out.ID),
		"commit_sha":  out.ID,
		"web_url":     out.WebURL,
		"success":     true,
		"error":       "",
	}, nil
}
