// Package upload uploads a file to Microsoft OneDrive using the simple upload API.
package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Upload File"
	Description  = "Upload a file to OneDrive (simple upload, under 4 MB)"
	Website      = "https://www.flomation.co"
	Icon         = "folder+arrow-up"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "filename", Type: core.ConnectionTypeString, Label: "Filename", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "File Content (text or base64)", Required: true},
	{Name: "folder_id", Type: core.ConnectionTypeString, Label: "Parent Folder ID"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Placeholder: "text/plain"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	filename := microsoft.OptStr("filename", inputs)
	if filename == "" {
		return microsoft.ErrorResult("filename is required")
	}

	contentConn := core.FindConnection("content", inputs)
	if contentConn == nil || contentConn.String() == nil || *contentConn.String() == "" {
		return microsoft.ErrorResult("content is required")
	}
	content := *contentConn.String()

	folderID := microsoft.OptStr("folder_id", inputs)
	contentType := microsoft.OptStr("content_type", inputs)
	if contentType == "" {
		contentType = "text/plain"
	}
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	tokens, err := microsoft.FetchTokens(flow, credential, "onedrive")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	var endpoint string
	if folderID != "" {
		endpoint = fmt.Sprintf("%s/me/drive/items/%s:/%s:/content",
			microsoft.GraphAPI, folderID, filename)
	} else {
		endpoint = fmt.Sprintf("%s/me/drive/root:/%s:/content",
			microsoft.GraphAPI, filename)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPut, endpoint, bytes.NewReader([]byte(content)))
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("upload request failed: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			microsoft.HandleAuthError(flow, token.Email, resp.StatusCode)
		}
		return microsoft.ErrorResult(fmt.Sprintf("upload returned %d: %s", resp.StatusCode, microsoft.TruncateBody(body)))
	}

	var result struct {
		ID     string `json:"id"`
		WebURL string `json:"webUrl"`
		Name   string `json:"name"`
		Size   int64  `json:"size"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse upload response: %v", err))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Uploaded %s (%d bytes) to OneDrive", result.Name, result.Size),
		"item_id":     result.ID,
		"web_url":     result.WebURL,
		"success":     true,
		"error":       "",
	}, nil
}
