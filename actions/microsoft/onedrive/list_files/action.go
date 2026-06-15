// Package list_files lists files and folders in a Microsoft OneDrive directory.
package list_files

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Files"
	Description  = "List files and folders in a OneDrive directory"
	Website      = "https://www.flomation.co"
	Icon         = "folder+list"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction

	selectFields = "id,name,size,lastModifiedDateTime,file,folder,webUrl"
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Folder ID", Placeholder: "Leave empty for root"},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "files", Type: core.ConnectionTypeString, Label: "Files (JSON)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "File Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)
	maxResults := microsoft.OptInt("max_results", inputs, 50)

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
	if itemID != "" {
		endpoint = fmt.Sprintf("%s/me/drive/items/%s/children?$top=%d&$select=%s",
			microsoft.GraphAPI, itemID, maxResults, selectFields)
	} else {
		endpoint = fmt.Sprintf("%s/me/drive/root/children?$top=%d&$select=%s",
			microsoft.GraphAPI, maxResults, selectFields)
	}

	status, body, err := microsoft.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	var resp struct {
		Value []struct {
			ID                   string `json:"id"`
			Name                 string `json:"name"`
			Size                 int64  `json:"size"`
			LastModifiedDateTime string `json:"lastModifiedDateTime"`
			WebURL               string `json:"webUrl"`
			File                 *struct {
				MimeType string `json:"mimeType"`
			} `json:"file"`
			Folder *struct {
				ChildCount int `json:"childCount"`
			} `json:"folder"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	filesJSON, _ := json.Marshal(resp.Value)

	var lines []string
	for _, item := range resp.Value {
		itemType := "file"
		if item.Folder != nil {
			itemType = "folder"
		}
		lines = append(lines, fmt.Sprintf("- %s (%s, %d bytes, modified %s)",
			item.Name, itemType, item.Size, item.LastModifiedDateTime))
	}
	summary := fmt.Sprintf("Found %d items:\n%s", len(resp.Value), strings.Join(lines, "\n"))

	return map[string]interface{}{
		"tool_result": summary,
		"files":       string(filesJSON),
		"count":       fmt.Sprintf("%d", len(resp.Value)),
		"success":     true,
		"error":       "",
	}, nil
}
