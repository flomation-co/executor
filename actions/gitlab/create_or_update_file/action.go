package gitlab_create_or_update_file

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Create or Update File"
	Description  = "Create or update a single file on a branch and commit it via the API (no git)"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+pencil"
	Date         = "29/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "branch", Type: core.ConnectionTypeString, Label: "Branch", Placeholder: "feature/my-change", Required: true},
	{Name: "file_path", Type: core.ConnectionTypeString, Label: "File Path", Placeholder: "path/to/file.md", Required: true},
	{Name: "content", Type: core.ConnectionTypeCode, Label: "File Content", Required: true},
	{Name: "commit_message", Type: core.ConnectionTypeString, Label: "Commit Message", Required: true},
	{Name: "author_name", Type: core.ConnectionTypeString, Label: "Author Name", Placeholder: "Optional — overrides the token owner"},
	{Name: "author_email", Type: core.ConnectionTypeString, Label: "Author Email", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "file_path", Type: core.ConnectionTypeString, Label: "File path"},
	{Name: "branch", Type: core.ConnectionTypeString, Label: "Branch"},
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
	filePath, err := gitlab.RequiredString("file_path", inputs)
	if err != nil {
		return nil, err
	}
	commitMessage, err := gitlab.RequiredString("commit_message", inputs)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"branch":         branch,
		"content":        gitlab.OptionalString("content", inputs),
		"commit_message": commitMessage,
	}
	if v := gitlab.OptionalString("author_name", inputs); v != "" {
		body["author_name"] = v
	}
	if v := gitlab.OptionalString("author_email", inputs); v != "" {
		body["author_email"] = v
	}

	// GitLab wants the file path URL-encoded WITH slashes escaped.
	enc := strings.ReplaceAll(url.PathEscape(filePath), "/", "%2F")
	apiPath := "/repository/files/" + enc

	// Update first; if the file doesn't exist yet GitLab returns 400, so fall
	// back to create. This gives one action true create-or-update semantics.
	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "PUT", apiPath, body)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to update file: %s", err)), nil
	}
	if resp.StatusCode == 400 {
		resp, err = gitlab.ProjectAPI(token, baseURL, projectID, "POST", apiPath, body)
		if err != nil {
			return gitlab.ErrorResult(fmt.Sprintf("Failed to create file: %s", err)), nil
		}
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var out struct {
		FilePath string `json:"file_path"`
		Branch   string `json:"branch"`
	}
	_ = json.Unmarshal(resp.Body, &out)
	if out.FilePath == "" {
		out.FilePath = filePath
	}
	if out.Branch == "" {
		out.Branch = branch
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Committed %s on %s", out.FilePath, out.Branch),
		"file_path":   out.FilePath,
		"branch":      out.Branch,
		"success":     true,
		"error":       "",
	}, nil
}
