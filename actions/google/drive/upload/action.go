// Package upload uploads a file to Google Drive using multipart upload.
package upload

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Upload to Drive"
	Description  = "Upload a file to Google Drive"
	Website      = "https://www.flomation.co"
	Icon         = "folder+arrow-up"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	// supportsAllDrives=true lets the upload land into a shared
	// drive when the parents in the metadata point at one — without
	// it, uploads scoped to a shared drive folder are rejected.
	uploadAPI = "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart&supportsAllDrives=true"
)

var Inputs = [...]core.Connection{
	{Name: "name", Type: core.ConnectionTypeString, Label: "File Name", Required: true, Placeholder: "report.txt"},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Text Content"},
	{Name: "base64_content", Type: core.ConnectionTypeString, Label: "Binary Content (base64)"},
	{Name: "mime_type", Type: core.ConnectionTypeString, Label: "MIME Type", Placeholder: "text/plain"},
	{Name: "folder_id", Type: core.ConnectionTypeString, Label: "Destination Folder ID"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File ID"},
	{Name: "file", Type: core.ConnectionTypeString, Label: "File Metadata (JSON)"},
	{Name: "web_link", Type: core.ConnectionTypeString, Label: "Web Link"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	name := google.OptStr("name", inputs)
	if name == "" {
		return google.ErrorResult("name is required")
	}

	textContent := google.OptStr("content", inputs)
	b64Content := google.OptStr("base64_content", inputs)
	mimeType := google.OptStr("mime_type", inputs)
	folderID := google.OptStr("folder_id", inputs)
	credential := google.OptStr("credential", inputs)
	account := google.OptStr("account", inputs)

	if textContent == "" && b64Content == "" {
		return google.ErrorResult("either content or base64_content is required")
	}

	tokens, err := google.FetchTokens(flow, credential)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	active := google.FilterTokens(tokens, account)
	if len(active) == 0 {
		return google.ErrorResult("no active Google account available")
	}
	token := active[0]

	// Resolve content
	var fileContent []byte
	if textContent != "" {
		fileContent = []byte(textContent)
		if mimeType == "" {
			mimeType = "text/plain"
		}
	} else {
		fileContent, err = base64.StdEncoding.DecodeString(b64Content)
		if err != nil {
			return google.ErrorResult(fmt.Sprintf("failed to decode base64 content: %v", err))
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	// Build metadata
	metadata := map[string]interface{}{
		"name": name,
	}
	if folderID != "" {
		metadata["parents"] = []string{folderID}
	}
	metadataJSON, _ := json.Marshal(metadata)

	status, body, err := google.DoMultipartUpload(flow, uploadAPI, token.AccessToken, metadataJSON, fileContent, mimeType)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status == 401 || status == 403 {
		google.HandleAuthError(flow, token.Email, status)
		return google.ErrorResult(fmt.Sprintf("Google API returned %d", status))
	}
	if status < 200 || status >= 300 {
		return google.ErrorResult(fmt.Sprintf("upload failed with status %d: %s", status, google.TruncateBody(body)))
	}

	var file map[string]interface{}
	if err := json.Unmarshal(body, &file); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	fileID, _ := file["id"].(string)
	webLink := fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Uploaded %s (%d bytes) {id:%s}", name, len(fileContent), fileID),
		"file_id":     fileID,
		"file":        string(body),
		"web_link":    webLink,
		"success":     true,
		"error":       "",
	}, nil
}
