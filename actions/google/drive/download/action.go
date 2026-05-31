// Package download downloads a file from Google Drive. For Google-native
// files (Docs, Sheets, Slides), exports to the requested format.
package download

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Download Drive File"
	Description  = "Download or export a file from Google Drive"
	Website      = "https://www.flomation.co"
	Icon         = "folder"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	driveAPI = "https://www.googleapis.com/drive/v3"
)

var exportMimeTypes = map[string]string{
	"text":  "text/plain",
	"pdf":   "application/pdf",
	"csv":   "text/csv",
	"docx":  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"xlsx":  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"pptx":  "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"html":  "text/html",
	"tsv":   "text/tab-separated-values",
}

var Inputs = [...]core.Connection{
	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File ID", Required: true},
	{
		Name:  "export_format",
		Type:  core.ConnectionTypeString,
		Label: "Export Format (for Google files)",
		Options: []core.ConnectionOption{
			{Name: "Plain Text", Value: "text"},
			{Name: "PDF", Value: "pdf"},
			{Name: "CSV (Sheets)", Value: "csv"},
			{Name: "DOCX (Docs)", Value: "docx"},
			{Name: "XLSX (Sheets)", Value: "xlsx"},
			{Name: "HTML", Value: "html"},
		},
	},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "File Content"},
	{Name: "content_base64", Type: core.ConnectionTypeString, Label: "Content (base64, for binary)"},
	{Name: "mime_type", Type: core.ConnectionTypeString, Label: "Content MIME Type"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	fileID := google.OptStr("file_id", inputs)
	if fileID == "" {
		return google.ErrorResult("file_id is required")
	}

	exportFormat := google.OptStr("export_format", inputs)
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

	// First, get file metadata to determine if it's a Google-native file
	metaURL := fmt.Sprintf("%s/files/%s?fields=mimeType,name,size", driveAPI, fileID)
	status, metaBody, err := google.DoRequest(flow, "GET", metaURL, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status == 401 || status == 403 {
		google.HandleAuthError(flow, token.Email, status)
		return google.ErrorResult(fmt.Sprintf("Google API returned %d", status))
	}
	if status == 404 {
		return google.ErrorResult(fmt.Sprintf("File not found: %s", fileID))
	}

	var meta struct {
		MimeType string `json:"mimeType"`
		Name     string `json:"name"`
	}
	if err := parseJSON(metaBody, &meta); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse metadata: %v", err))
	}

	isGoogleNative := strings.HasPrefix(meta.MimeType, "application/vnd.google-apps.")

	var downloadURL string
	var responseMimeType string

	if isGoogleNative {
		// Export Google-native files
		if exportFormat == "" {
			exportFormat = "text" // Default to plain text
		}
		exportMime, ok := exportMimeTypes[exportFormat]
		if !ok {
			return google.ErrorResult(fmt.Sprintf("unsupported export format: %s", exportFormat))
		}
		downloadURL = fmt.Sprintf("%s/files/%s/export?mimeType=%s", driveAPI, fileID, url.QueryEscape(exportMime))
		responseMimeType = exportMime
	} else {
		// Direct download for non-Google files
		downloadURL = fmt.Sprintf("%s/files/%s?alt=media", driveAPI, fileID)
		responseMimeType = meta.MimeType
	}

	status, body, err := google.DoRequestLong(flow, "GET", downloadURL, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status != 200 {
		return google.ErrorResult(fmt.Sprintf("download failed with status %d: %s", status, google.TruncateBody(body)))
	}

	// Determine if content is text or binary
	isText := strings.HasPrefix(responseMimeType, "text/") ||
		responseMimeType == "application/json" ||
		responseMimeType == "application/xml"

	result := map[string]interface{}{
		"tool_result":    fmt.Sprintf("Downloaded %s (%d bytes)", meta.Name, len(body)),
		"mime_type":      responseMimeType,
		"success":        true,
		"error":          "",
	}

	if isText {
		result["content"] = string(body)
		result["content_base64"] = ""
	} else {
		result["content"] = ""
		result["content_base64"] = base64.StdEncoding.EncodeToString(body)
	}

	return result, nil
}

func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v) // #nosec G107
}
