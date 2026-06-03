// Package list_folders lists mail folders in a Microsoft Outlook mailbox.
package list_folders

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
	Name         = "List Folders"
	Description  = "List mail folders in an Outlook mailbox"
	Website      = "https://www.flomation.co"
	Icon         = "folder"
	Date         = "03/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_OUTLOOK}"},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "folders", Type: core.ConnectionTypeString, Label: "Folders (JSON)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Folder Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)
	maxResults := microsoft.OptInt("max_results", inputs, 50)

	tokens, err := microsoft.FetchTokens(flow, credential, "mail_read")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	endpoint := fmt.Sprintf("%s/me/mailFolders?$top=%d", microsoft.GraphAPI, maxResults)

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
			ID               string `json:"id"`
			DisplayName      string `json:"displayName"`
			TotalItemCount   int    `json:"totalItemCount"`
			UnreadItemCount  int    `json:"unreadItemCount"`
			ParentFolderID   string `json:"parentFolderId"`
			ChildFolderCount int    `json:"childFolderCount"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	foldersJSON, _ := json.Marshal(resp.Value)

	var names []string
	for _, f := range resp.Value {
		names = append(names, fmt.Sprintf("%s (%d messages, %d unread)", f.DisplayName, f.TotalItemCount, f.UnreadItemCount))
	}
	summary := fmt.Sprintf("Found %d folders:\n%s", len(resp.Value), strings.Join(names, "\n"))

	return map[string]interface{}{
		"tool_result": summary,
		"folders":     string(foldersJSON),
		"count":       fmt.Sprintf("%d", len(resp.Value)),
		"success":     true,
		"error":       "",
	}, nil
}
