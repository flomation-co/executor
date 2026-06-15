// Package upload uploads a PowerPoint presentation to Microsoft OneDrive.
package upload

import (
	"bytes"
	"encoding/base64"
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
	Name         = "Upload Presentation"
	Description  = "Upload a PowerPoint presentation to OneDrive"
	Website      = "https://www.flomation.co"
	Icon         = "display+arrow-up"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction

	pptxContentType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
)

var Inputs = [...]core.Connection{
	{Name: "filename", Type: core.ConnectionTypeString, Label: "Filename", Required: true, Placeholder: "presentation.pptx"},
	{Name: "content", Type: core.ConnectionTypeText, Label: "File Content (base64)", Required: true},
	{Name: "folder_path", Type: core.ConnectionTypeString, Label: "Folder Path"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
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
	rawContent := *contentConn.String()

	folderPath := microsoft.OptStr("folder_path", inputs)
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

	// Decode the base64 content.
	decoded, err := base64.StdEncoding.DecodeString(rawContent)
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to decode base64 content: %v", err))
	}

	var endpoint string
	if folderPath != "" {
		endpoint = fmt.Sprintf("%s/me/drive/root:/%s/%s:/content",
			microsoft.GraphAPI, folderPath, filename)
	} else {
		endpoint = fmt.Sprintf("%s/me/drive/root:/%s:/content",
			microsoft.GraphAPI, filename)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPut, endpoint, bytes.NewReader(decoded))
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", pptxContentType)

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
