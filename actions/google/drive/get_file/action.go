// Package get_file retrieves metadata for a specific Google Drive file.
package get_file

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Drive File"
	Description  = "Get metadata for a Google Drive file"
	Website      = "https://www.flomation.co"
	Icon         = "folder"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	driveAPI = "https://www.googleapis.com/drive/v3"
)

var Inputs = [...]core.Connection{
	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "file", Type: core.ConnectionTypeString, Label: "File Metadata (JSON)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "File Name"},
	{Name: "mime_type", Type: core.ConnectionTypeString, Label: "MIME Type"},
	{Name: "size", Type: core.ConnectionTypeString, Label: "File Size"},
	{Name: "web_link", Type: core.ConnectionTypeString, Label: "Web Link"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	fileID := google.OptStr("file_id", inputs)
	if fileID == "" {
		return google.ErrorResult("file_id is required")
	}

	credential := google.OptStr("credential", inputs)
	account := google.OptStr("account", inputs)

	tokens, err := google.FetchTokens(flow, credential)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	active := google.FilterTokens(tokens, account)
	if len(active) == 0 {
		return google.ErrorResult("no active Google account available")
	}

	endpoint := fmt.Sprintf("%s/files/%s?fields=id,name,mimeType,size,modifiedTime,createdTime,owners,permissions,webViewLink,parents,description",
		driveAPI, fileID)

	token := active[0]
	status, body, err := google.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status == 401 || status == 403 {
		google.HandleAuthError(flow, token.Email, status)
		return google.ErrorResult(fmt.Sprintf("Google API returned %d", status))
	}
	if status == 404 {
		return google.ErrorResult(fmt.Sprintf("File not found: %s", fileID))
	}
	if status != 200 {
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	var file map[string]interface{}
	if err := json.Unmarshal(body, &file); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	name, _ := file["name"].(string)
	mimeType, _ := file["mimeType"].(string)
	size, _ := file["size"].(string)
	webLink, _ := file["webViewLink"].(string)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%s (%s) — %s", name, mimeType, webLink),
		"file":        string(body),
		"name":        name,
		"mime_type":   mimeType,
		"size":        size,
		"web_link":    webLink,
		"success":     true,
		"error":       "",
	}, nil
}
