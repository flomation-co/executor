package gitlab_get_file

import (
	"encoding/base64"
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
	Name         = "GitLab Get File"
	Description  = "Read a single file's contents from a branch, tag or commit via the API (no git)"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+file-lines"
	Date         = "29/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "file_path", Type: core.ConnectionTypeString, Label: "File Path", Placeholder: "path/to/file.json", Required: true},
	{Name: "ref", Type: core.ConnectionTypeString, Label: "Ref", Placeholder: "main — branch, tag or commit SHA to read from", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "File content"},
	{Name: "file_path", Type: core.ConnectionTypeString, Label: "File path"},
	{Name: "file_name", Type: core.ConnectionTypeString, Label: "File name"},
	{Name: "size", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "sha", Type: core.ConnectionTypeString, Label: "Content SHA-256"},
	{Name: "blob_id", Type: core.ConnectionTypeString, Label: "Blob ID"},
	{Name: "last_commit_id", Type: core.ConnectionTypeString, Label: "Last commit SHA"},
	{Name: "ref", Type: core.ConnectionTypeString, Label: "Ref"},
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
	filePath, err := gitlab.RequiredString("file_path", inputs)
	if err != nil {
		return nil, err
	}
	ref, err := gitlab.RequiredString("ref", inputs)
	if err != nil {
		return nil, err
	}

	// GitLab wants the file path URL-encoded WITH slashes escaped, and the ref
	// as a query param on GET /repository/files/:path.
	enc := strings.ReplaceAll(url.PathEscape(filePath), "/", "%2F")
	apiPath := fmt.Sprintf("/repository/files/%s?ref=%s", enc, url.QueryEscape(ref))

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", apiPath, nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to fetch file: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var out struct {
		FileName      string `json:"file_name"`
		FilePath      string `json:"file_path"`
		Size          int64  `json:"size"`
		Encoding      string `json:"encoding"`
		Content       string `json:"content"`
		ContentSHA256 string `json:"content_sha256"`
		Ref           string `json:"ref"`
		BlobID        string `json:"blob_id"`
		LastCommitID  string `json:"last_commit_id"`
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	content := out.Content
	if strings.EqualFold(out.Encoding, "base64") {
		decoded, decErr := base64.StdEncoding.DecodeString(out.Content)
		if decErr != nil {
			return gitlab.ErrorResult(fmt.Sprintf("Failed to decode file content: %s", decErr)), nil
		}
		content = string(decoded)
	}

	if out.FilePath == "" {
		out.FilePath = filePath
	}
	if out.Ref == "" {
		out.Ref = ref
	}

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Read %s (%d bytes) from %s", out.FilePath, out.Size, out.Ref),
		"content":        content,
		"file_path":      out.FilePath,
		"file_name":      out.FileName,
		"size":           out.Size,
		"sha":            out.ContentSHA256,
		"blob_id":        out.BlobID,
		"last_commit_id": out.LastCommitID,
		"ref":            out.Ref,
		"success":        true,
		"error":          "",
	}, nil
}
