// Package copy_file copies a file or folder to a different location in Microsoft OneDrive.
package copy_file

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Copy Item"
	Description  = "Copy a file or folder to a different OneDrive location"
	Website      = "https://www.flomation.co"
	Icon         = "folder+file-export"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Required: true},
	{Name: "destination_folder_id", Type: core.ConnectionTypeString, Label: "Destination Folder ID", Required: true},
	{Name: "new_name", Type: core.ConnectionTypeString, Label: "New Name"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	if itemID == "" {
		return microsoft.ErrorResult("item_id is required")
	}
	destFolderID := microsoft.OptStr("destination_folder_id", inputs)
	if destFolderID == "" {
		return microsoft.ErrorResult("destination_folder_id is required")
	}

	newName := microsoft.OptStr("new_name", inputs)
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

	endpoint := fmt.Sprintf("%s/me/drive/items/%s/copy", microsoft.GraphAPI, itemID)

	payload := map[string]interface{}{
		"parentReference": map[string]string{
			"id": destFolderID,
		},
	}
	if newName != "" {
		payload["name"] = newName
	}
	body, _ := json.Marshal(payload)

	status, respBody, err := microsoft.DoRequest(flow, "POST", endpoint, token.AccessToken, body)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	// Copy returns 202 Accepted for asynchronous operations.
	if status != 202 && (status < 200 || status >= 300) {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(respBody)))
	}

	summary := fmt.Sprintf("Copy of item %s initiated to folder %s", itemID, destFolderID)
	if newName != "" {
		summary += fmt.Sprintf(" as '%s'", newName)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}, nil
}
