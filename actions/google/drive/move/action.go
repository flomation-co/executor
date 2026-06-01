// Package move moves a file to a different folder in Google Drive.
package move

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Move Drive File"
	Description  = "Move a file to a different folder in Google Drive"
	Website      = "https://www.flomation.co"
	Icon         = "folder"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	driveAPI = "https://www.googleapis.com/drive/v3"
)

var Inputs = [...]core.Connection{
	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File ID", Required: true},
	{Name: "destination_folder_id", Type: core.ConnectionTypeString, Label: "Destination Folder ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "file", Type: core.ConnectionTypeString, Label: "File Metadata (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	fileID := google.OptStr("file_id", inputs)
	if fileID == "" {
		return google.ErrorResult("file_id is required")
	}
	destFolder := google.OptStr("destination_folder_id", inputs)
	if destFolder == "" {
		return google.ErrorResult("destination_folder_id is required")
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
	token := active[0]

	// Get current parents
	metaURL := fmt.Sprintf("%s/files/%s?fields=parents", driveAPI, fileID)
	status, metaBody, err := google.DoRequest(flow, "GET", metaURL, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status != 200 {
		return google.ErrorResult(fmt.Sprintf("failed to get file metadata: %s", google.TruncateBody(metaBody)))
	}

	var meta struct {
		Parents []string `json:"parents"`
	}
	_ = json.Unmarshal(metaBody, &meta)

	// Move: add new parent, remove old parents
	removeParents := ""
	if len(meta.Parents) > 0 {
		for i, p := range meta.Parents {
			if i > 0 {
				removeParents += ","
			}
			removeParents += p
		}
	}

	endpoint := fmt.Sprintf("%s/files/%s?addParents=%s&removeParents=%s&fields=id,name,parents,webViewLink",
		driveAPI, fileID, destFolder, removeParents)

	status, body, err := google.DoRequest(flow, "PATCH", endpoint, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	var file map[string]interface{}
	_ = json.Unmarshal(body, &file)
	name, _ := file["name"].(string)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Moved '%s' to folder %s", name, destFolder),
		"file":        string(body),
		"success":     true,
		"error":       "",
	}, nil
}
