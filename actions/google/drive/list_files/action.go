// Package list_files lists files and folders in Google Drive.
package list_files

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
	Name         = "List Drive Files"
	Description  = "List files and folders in Google Drive"
	Website      = "https://www.flomation.co"
	Icon         = "folder+list"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	driveAPI = "https://www.googleapis.com/drive/v3"
)

var Inputs = [...]core.Connection{
	{Name: "folder_id", Type: core.ConnectionTypeString, Label: "Folder ID", Placeholder: "root"},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Drive Query Filter", Placeholder: "name contains 'report'"},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Max Results", Placeholder: "20"},
	{Name: "include_trashed", Type: core.ConnectionTypeBoolean, Label: "Include Trashed"},
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
	credential := google.OptStr("credential", inputs)
	account := google.OptStr("account", inputs)
	folderID := google.OptStr("folder_id", inputs)
	query := google.OptStr("query", inputs)
	maxResults := google.OptInt("max_results", inputs, 20)
	includeTrashed := google.OptBool("include_trashed", inputs)

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
	if folderID != "" {
		qParts = append(qParts, fmt.Sprintf("'%s' in parents", folderID))
	}
	if query != "" {
		qParts = append(qParts, query)
	}
	if !includeTrashed {
		qParts = append(qParts, "trashed = false")
	}

	q := strings.Join(qParts, " and ")

	params := url.Values{}
	if q != "" {
		params.Set("q", q)
	}
	params.Set("pageSize", fmt.Sprintf("%d", maxResults))
	params.Set("fields", "files(id,name,mimeType,size,modifiedTime,createdTime,parents,webViewLink,iconLink)")
	params.Set("orderBy", "modifiedTime desc")

	endpoint := fmt.Sprintf("%s/files?%s", driveAPI, params.Encode())

	token := active[0]
	status, body, err := google.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status == 401 || status == 403 {
		google.HandleAuthError(flow, token.Email, status)
		return google.ErrorResult(fmt.Sprintf("Google API returned %d — account may need reconnection", status))
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

	// Build human-readable summary
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Found %d file(s)", count))
	if folderID != "" {
		summary.WriteString(fmt.Sprintf(" in folder %s", folderID))
	}
	summary.WriteString(":\n")
	for _, f := range resp.Files {
		name, _ := f["name"].(string)
		mimeType, _ := f["mimeType"].(string)
		id, _ := f["id"].(string)
		typeLabel := friendlyType(mimeType)
		summary.WriteString(fmt.Sprintf("  - %s (%s) {id:%s}\n", name, typeLabel, id))
	}

	return map[string]interface{}{
		"tool_result": summary.String(),
		"files":       string(filesJSON),
		"count":       count,
		"success":     true,
		"error":       "",
	}, nil
}

func friendlyType(mimeType string) string {
	switch mimeType {
	case "application/vnd.google-apps.folder":
		return "folder"
	case "application/vnd.google-apps.document":
		return "Google Doc"
	case "application/vnd.google-apps.spreadsheet":
		return "Google Sheet"
	case "application/vnd.google-apps.presentation":
		return "Google Slides"
	case "application/pdf":
		return "PDF"
	default:
		if strings.HasPrefix(mimeType, "image/") {
			return "image"
		}
		return mimeType
	}
}
