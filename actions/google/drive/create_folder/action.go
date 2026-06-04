// Package create_folder creates a new folder in Google Drive.
package create_folder

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Drive Folder"
	Description  = "Create a new folder in Google Drive"
	Website      = "https://www.flomation.co"
	Icon         = "folder+plus"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	driveAPI = "https://www.googleapis.com/drive/v3"
)

var Inputs = [...]core.Connection{
	{Name: "name", Type: core.ConnectionTypeString, Label: "Folder Name", Required: true},
	{Name: "parent_folder_id", Type: core.ConnectionTypeString, Label: "Parent Folder ID", Placeholder: "root"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "folder_id", Type: core.ConnectionTypeString, Label: "Folder ID"},
	{Name: "web_link", Type: core.ConnectionTypeString, Label: "Web Link"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	name := google.OptStr("name", inputs)
	if name == "" {
		return google.ErrorResult("name is required")
	}

	parentID := google.OptStr("parent_folder_id", inputs)
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
	token := active[0]

	metadata := map[string]interface{}{
		"name":     name,
		"mimeType": "application/vnd.google-apps.folder",
	}
	if parentID != "" {
		metadata["parents"] = []string{parentID}
	}
	body, _ := json.Marshal(metadata)

	endpoint := fmt.Sprintf("%s/files", driveAPI)
	status, respBody, err := google.DoRequest(flow, "POST", endpoint, token.AccessToken, body)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status == 401 || status == 403 {
		google.HandleAuthError(flow, token.Email, status)
		return google.ErrorResult(fmt.Sprintf("Google API returned %d", status))
	}
	if status < 200 || status >= 300 {
		return google.ErrorResult(fmt.Sprintf("failed to create folder: %s", google.TruncateBody(respBody)))
	}

	var folder map[string]interface{}
	if err := json.Unmarshal(respBody, &folder); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	folderID, _ := folder["id"].(string)
	webLink := fmt.Sprintf("https://drive.google.com/drive/folders/%s", folderID)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created folder '%s' {id:%s}", name, folderID),
		"folder_id":   folderID,
		"web_link":    webLink,
		"success":     true,
		"error":       "",
	}, nil
}
