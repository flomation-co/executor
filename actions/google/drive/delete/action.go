// Package delete deletes or trashes a file in Google Drive.
package delete

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Delete Drive File"
	Description  = "Delete or trash a file in Google Drive"
	Website      = "https://www.flomation.co"
	Icon         = "folder"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	driveAPI = "https://www.googleapis.com/drive/v3"
)

var Inputs = [...]core.Connection{
	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File ID", Required: true},
	{Name: "permanent", Type: core.ConnectionTypeBoolean, Label: "Permanently Delete"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	fileID := google.OptStr("file_id", inputs)
	if fileID == "" {
		return google.ErrorResult("file_id is required")
	}

	permanent := google.OptBool("permanent", inputs)
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

	var status int
	var body []byte

	if permanent {
		endpoint := fmt.Sprintf("%s/files/%s", driveAPI, fileID)
		status, body, err = google.DoRequest(flow, "DELETE", endpoint, token.AccessToken, nil)
	} else {
		// Trash: update the trashed property
		endpoint := fmt.Sprintf("%s/files/%s", driveAPI, fileID)
		payload, _ := json.Marshal(map[string]interface{}{"trashed": true})
		status, body, err = google.DoRequest(flow, "PATCH", endpoint, token.AccessToken, payload)
	}

	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status == 401 || status == 403 {
		google.HandleAuthError(flow, token.Email, status)
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}
	// 204 No Content (permanent delete) or 200 (trash) are both success
	if status != 200 && status != 204 {
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	action := "Trashed"
	if permanent {
		action = "Permanently deleted"
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%s file %s", action, fileID),
		"success":     true,
		"error":       "",
	}, nil
}
