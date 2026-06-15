// Package get_metadata retrieves metadata for a Google Docs document.
package get_metadata

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Document Metadata"
	Description  = "Get metadata for a Google Docs document"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+eye"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	docsAPI = "https://docs.googleapis.com/v1/documents"
)

var Inputs = [...]core.Connection{
	{Name: "document_id", Type: core.ConnectionTypeString, Label: "Document ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "revision_id", Type: core.ConnectionTypeString, Label: "Revision ID"},
	{Name: "document", Type: core.ConnectionTypeString, Label: "Metadata (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	docID := google.OptStr("document_id", inputs)
	if docID == "" {
		return google.ErrorResult("document_id is required")
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

	endpoint := fmt.Sprintf("%s/%s?fields=documentId,title,revisionId", docsAPI, docID)

	status, body, err := google.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	var doc struct {
		DocumentID string `json:"documentId"`
		Title      string `json:"title"`
		RevisionID string `json:"revisionId"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("'%s' (revision: %s)", doc.Title, doc.RevisionID),
		"title":       doc.Title,
		"revision_id": doc.RevisionID,
		"document":    string(body),
		"success":     true,
		"error":       "",
	}, nil
}
