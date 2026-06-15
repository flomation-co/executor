// Package create_folder creates a new folder in Microsoft OneDrive.
package create_folder

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Folder"
	Description  = "Create a new folder in OneDrive"
	Website      = "https://www.flomation.co"
	Icon         = "folder+plus"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "name", Type: core.ConnectionTypeString, Label: "Folder Name", Required: true},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Parent Folder ID", Placeholder: "Leave empty for root"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Folder ID"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	folderName := microsoft.OptStr("name", inputs)
	if folderName == "" {
		return microsoft.ErrorResult("name is required")
	}

	parentID := microsoft.OptStr("parent_id", inputs)
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
	if parentID != "" {
		endpoint = fmt.Sprintf("%s/me/drive/items/%s/children", microsoft.GraphAPI, parentID)
	} else {
		endpoint = fmt.Sprintf("%s/me/drive/root/children", microsoft.GraphAPI)
	}

	payload := map[string]interface{}{
		"name":                              folderName,
		"folder":                            map[string]interface{}{},
		"@microsoft.graph.conflictBehavior": "rename",
	}
	body, _ := json.Marshal(payload)

	status, respBody, err := microsoft.DoRequest(flow, "POST", endpoint, token.AccessToken, body)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(respBody)))
	}

	var result struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		WebURL string `json:"webUrl"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created folder '%s' in OneDrive", result.Name),
		"item_id":     result.ID,
		"web_url":     result.WebURL,
		"success":     true,
		"error":       "",
	}, nil
}
