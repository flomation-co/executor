// Package import_doc converts a Drive file (e.g. an uploaded .docx) into a
// native Google Doc so agents can edit it live via the Docs API — the same
// collaborative, in-place model as Google Sheets. This is a one-time convert:
// the returned document_id is the canonical editable doc; there is no
// download/edit/re-upload round-trip.
package import_doc

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Convert to Google Doc"
	Description  = "Convert a Drive file (e.g. an uploaded .docx) into a native, editable Google Doc"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+arrow-right-arrow-left"
	Date         = "10/08/2026"
	Type         = core.ActionTypeAction

	driveAPI = "https://www.googleapis.com/drive/v3"

	// googleDocMime is the target type Drive converts an importable file to.
	googleDocMime = "application/vnd.google-apps.document"
)

var Inputs = [...]core.Connection{
	{Name: "file_id", Type: core.ConnectionTypeString, Label: "Source Drive File ID (.docx, .odt, etc.)", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name for the new Google Doc (defaults to the source name)"},
	{Name: "folder_id", Type: core.ConnectionTypeString, Label: "Destination Folder ID (defaults to the source's location)"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "document_id", Type: core.ConnectionTypeString, Label: "New Google Doc ID (use with the Docs actions)"},
	{Name: "web_link", Type: core.ConnectionTypeString, Label: "Web Link"},
	{Name: "already_native", Type: core.ConnectionTypeBoolean, Label: "Source was already a Google Doc"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	fileID := google.OptStr("file_id", inputs)
	if fileID == "" {
		return google.ErrorResult("file_id is required")
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

	// Look up the source so we can (a) short-circuit if it's already a Google
	// Doc, and (b) default the new name to the source name.
	metaURL := google.AppendDriveSingleFileQS(fmt.Sprintf("%s/files/%s?fields=id,name,mimeType", driveAPI, fileID))
	status, body, err := google.DoRequest(flow, "GET", metaURL, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("could not read source file (Google API returned %d): %s", status, google.TruncateBody(body)))
	}
	var meta struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		MimeType string `json:"mimeType"`
	}
	_ = json.Unmarshal(body, &meta)

	// Already a native Google Doc — nothing to convert.
	if meta.MimeType == googleDocMime {
		return map[string]interface{}{
			"tool_result":    fmt.Sprintf("'%s' is already a Google Doc — edit it directly with document_id %s.", meta.Name, meta.ID),
			"document_id":    meta.ID,
			"web_link":       fmt.Sprintf("https://docs.google.com/document/d/%s/edit", meta.ID),
			"already_native": true,
			"success":        true,
			"error":          "",
		}, nil
	}

	newName := google.OptStr("name", inputs)
	if newName == "" {
		newName = meta.Name
	}
	copyBody := map[string]interface{}{
		"name":     newName,
		"mimeType": googleDocMime, // Drive converts the importable file on copy.
	}
	if folderID := google.OptStr("folder_id", inputs); folderID != "" {
		copyBody["parents"] = []string{folderID}
	}
	payload, _ := json.Marshal(copyBody)

	copyURL := google.AppendDriveSingleFileQS(fmt.Sprintf("%s/files/%s/copy?fields=id,name,mimeType,webViewLink", driveAPI, fileID))
	status, body, err = google.DoRequest(flow, "POST", copyURL, token.AccessToken, payload)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("conversion failed (Google API returned %d): %s", status, google.TruncateBody(body)))
	}
	var created struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		WebViewLink string `json:"webViewLink"`
	}
	_ = json.Unmarshal(body, &created)

	link := created.WebViewLink
	if link == "" {
		link = fmt.Sprintf("https://docs.google.com/document/d/%s/edit", created.ID)
	}

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Converted '%s' to a native Google Doc. Edit it live with document_id %s (read, insert text, fill tables, replace text).", created.Name, created.ID),
		"document_id":    created.ID,
		"web_link":       link,
		"already_native": false,
		"success":        true,
		"error":          "",
	}, nil
}
