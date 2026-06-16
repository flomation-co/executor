// Package search searches for files in Google Drive by name, type, or content.
package search

import (
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
	Name         = "Search Drive"
	Description  = "Search for files in Google Drive"
	Website      = "https://www.flomation.co"
	Icon         = "folder+magnifying-glass"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	driveAPI = "https://www.googleapis.com/drive/v3"
)

var mimeTypes = map[string]string{
	"document":     "application/vnd.google-apps.document",
	"spreadsheet":  "application/vnd.google-apps.spreadsheet",
	"presentation": "application/vnd.google-apps.presentation",
	"folder":       "application/vnd.google-apps.folder",
	"pdf":          "application/pdf",
	"image":        "image/",
}

var Inputs = [...]core.Connection{
	{Name: "name", Type: core.ConnectionTypeString, Label: "File Name (contains)", Placeholder: "report"},
	{
		Name:  "mime_type",
		Type:  core.ConnectionTypeString,
		Label: "File Type",
		Options: []core.ConnectionOption{
			{Name: "Any", Value: ""},
			{Name: "Document", Value: "document"},
			{Name: "Spreadsheet", Value: "spreadsheet"},
			{Name: "Presentation", Value: "presentation"},
			{Name: "Folder", Value: "folder"},
			{Name: "PDF", Value: "pdf"},
		},
	},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content Contains", Placeholder: "keyword"},
	{
		// scope narrows the search corpus. Default is "all" which
		// covers My Drive + shared-with-me + shared drives — the
		// behaviour most agents expect ("find anything the user
		// can see"). Narrow to my_drive when you specifically
		// only want files the user owns. Narrow to shared_with_me
		// to discover items individually shared to the user.
		Name:  "scope",
		Type:  core.ConnectionTypeString,
		Label: "Search Scope",
		Options: []core.ConnectionOption{
			{Name: "Everything I can access (default)", Value: "all"},
			{Name: "My Drive only", Value: "my_drive"},
			{Name: "Shared with me only", Value: "shared_with_me"},
			{Name: "Shared drives only", Value: "shared_drives"},
		},
	},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Max Results", Placeholder: "20"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "files", Type: core.ConnectionTypeString, Label: "Files (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "File Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	nameFilter := google.OptStr("name", inputs)
	mimeTypeKey := google.OptStr("mime_type", inputs)
	contentFilter := google.OptStr("content", inputs)
	scope := google.OptStr("scope", inputs)
	maxResults := google.OptInt("max_results", inputs, 20)
	credential := google.OptStr("credential", inputs)
	account := google.OptStr("account", inputs)

	if nameFilter == "" && mimeTypeKey == "" && contentFilter == "" {
		return google.ErrorResult("at least one search criterion (name, mime_type, or content) is required")
	}

	if maxResults > 100 {
		maxResults = 100
	}

	tokens, err := google.FetchTokens(flow, credential)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	active := google.FilterTokens(tokens, account)
	if len(active) == 0 {
		return google.ErrorResult("no active Google account available")
	}

	// Build query
	var qParts []string
	if nameFilter != "" {
		qParts = append(qParts, fmt.Sprintf("name contains '%s'", nameFilter))
	}
	if mimeTypeKey != "" {
		if mime, ok := mimeTypes[mimeTypeKey]; ok {
			if mimeTypeKey == "image" {
				qParts = append(qParts, fmt.Sprintf("mimeType contains '%s'", mime))
			} else {
				qParts = append(qParts, fmt.Sprintf("mimeType = '%s'", mime))
			}
		}
	}
	if contentFilter != "" {
		qParts = append(qParts, fmt.Sprintf("fullText contains '%s'", contentFilter))
	}
	qParts = append(qParts, "trashed = false")

	// Scope translates user-facing options into Google Drive API
	// corpora + q-clause modifiers. See AppendDriveListParams for
	// the explanation of corpora vs my-drive vs shared. The
	// "shared_with_me" path is the only one that needs an extra
	// q-clause — the others are pure corpus selection.
	corpora := "allDrives"
	switch scope {
	case "my_drive":
		corpora = "user"
	case "shared_drives":
		corpora = "drive"
	case "shared_with_me":
		corpora = "user"
		qParts = append(qParts, "sharedWithMe = true")
	}

	params := url.Values{}
	params.Set("q", strings.Join(qParts, " and "))
	params.Set("pageSize", fmt.Sprintf("%d", maxResults))
	params.Set("fields", "files(id,name,mimeType,size,modifiedTime,webViewLink)")
	params.Set("orderBy", "modifiedTime desc")
	google.AppendDriveListParams(params, corpora)

	endpoint := fmt.Sprintf("%s/files?%s", driveAPI, params.Encode())

	token := active[0]
	status, body, err := google.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status == 401 || status == 403 {
		google.HandleAuthError(flow, token.Email, status)
		return google.ErrorResult(fmt.Sprintf("Google API returned %d", status))
	}
	if status != 200 {
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	var resp struct {
		Files []map[string]interface{} `json:"files"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	filesJSON, _ := json.Marshal(resp.Files)
	count := int64(len(resp.Files))

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Found %d file(s):\n", count))
	for _, f := range resp.Files {
		name, _ := f["name"].(string)
		id, _ := f["id"].(string)
		summary.WriteString(fmt.Sprintf("  - %s {id:%s}\n", name, id))
	}

	return map[string]interface{}{
		"tool_result": summary.String(),
		"files":       string(filesJSON),
		"count":       count,
		"success":     true,
		"error":       "",
	}, nil
}
