// Package copy copies a file in Google Drive.
package copy

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Copy Drive File"
	Description  = "Copy a file in Google Drive"
	Website      = "https://www.flomation.co"
	Icon         = "folder+file-export"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	driveAPI = "https://www.googleapis.com/drive/v3"
)

var Inputs = [...]core.Connection{
	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File ID", Required: true},
	{Name: "new_name", Type: core.ConnectionTypeString, Label: "New Name"},
	{Name: "destination_folder_id", Type: core.ConnectionTypeString, Label: "Destination Folder ID"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "new_file_id", Type: core.ConnectionTypeString, Label: "New File ID"},
	{Name: "file", Type: core.ConnectionTypeString, Label: "File Metadata (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	fileID := google.OptStr("file_id", inputs)
	if fileID == "" {
		return google.ErrorResult("file_id is required")
	}

	newName := google.OptStr("new_name", inputs)
	destFolder := google.OptStr("destination_folder_id", inputs)
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

	payload := make(map[string]interface{})
	if newName != "" {
		payload["name"] = newName
	}
	if destFolder != "" {
		payload["parents"] = []string{destFolder}
	}

	body, _ := json.Marshal(payload)
	endpoint := fmt.Sprintf("%s/files/%s/copy?fields=id,name,webViewLink", driveAPI, fileID)

	status, respBody, err := google.DoRequest(flow, "POST", endpoint, token.AccessToken, body)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(respBody)))
	}

	var file map[string]interface{}
	_ = json.Unmarshal(respBody, &file)
	newFileID, _ := file["id"].(string)
	name, _ := file["name"].(string)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Copied to '%s' {id:%s}", name, newFileID),
		"new_file_id": newFileID,
		"file":        string(respBody),
		"success":     true,
		"error":       "",
	}, nil
}
