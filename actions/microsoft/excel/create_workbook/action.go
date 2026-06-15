// Package create_workbook creates a new Excel workbook in OneDrive.
package create_workbook

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
	Name         = "Create Workbook"
	Description  = "Create a new Excel workbook in OneDrive"
	Website      = "https://www.flomation.co"
	Icon         = "table+plus"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction

	xlsxMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

var Inputs = [...]core.Connection{
	{Name: "filename", Type: core.ConnectionTypeString, Label: "Filename", Required: true, Placeholder: "report.xlsx"},
	{Name: "folder_path", Type: core.ConnectionTypeString, Label: "Folder Path", Placeholder: "Documents/Reports"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_EXCEL}"},
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
	folderPath := microsoft.OptStr("folder_path", inputs)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	if filename == "" {
		return microsoft.ErrorResult("filename is required")
	}

	tokens, err := microsoft.FetchTokens(flow, credential, "excel")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	var endpoint string
	if folderPath != "" {
		endpoint = fmt.Sprintf("%s/me/drive/root:/%s/%s:/content",
			microsoft.GraphAPI, folderPath, filename)
	} else {
		endpoint = fmt.Sprintf("%s/me/drive/root:/%s:/content",
			microsoft.GraphAPI, filename)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPut, endpoint, bytes.NewReader([]byte{}))
	if err != nil {
		return microsoft.ErrorResult("failed to create request: " + err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", xlsxMIME)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return microsoft.ErrorResult("request failed: " + err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			microsoft.HandleAuthError(flow, token.Email, resp.StatusCode)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", resp.StatusCode, microsoft.TruncateBody(respBody)))
	}

	var result struct {
		ID     string `json:"id"`
		WebURL string `json:"webUrl"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return microsoft.ErrorResult("failed to parse response: " + err.Error())
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created workbook: %s (ID: %s)", filename, result.ID),
		"item_id":     result.ID,
		"web_url":     result.WebURL,
		"success":     true,
		"error":       "",
	}, nil
}
