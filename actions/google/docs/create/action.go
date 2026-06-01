// Package create creates a new Google Docs document.
package create

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Document"
	Description  = "Create a new Google Docs document"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	docsAPI = "https://docs.googleapis.com/v1/documents"
)

var Inputs = [...]core.Connection{
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Required: true, Placeholder: "My Document"},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Initial Content"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "document_id", Type: core.ConnectionTypeString, Label: "Document ID"},
	{Name: "document_url", Type: core.ConnectionTypeString, Label: "Document URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	title := google.OptStr("title", inputs)
	if title == "" {
		return google.ErrorResult("title is required")
	}
	content := google.OptStr("content", inputs)

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

	// Step 1: Create empty document
	createPayload, _ := json.Marshal(map[string]string{"title": title})
	status, body, err := google.DoRequest(flow, "POST", docsAPI, token.AccessToken, createPayload)
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
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	// Step 2: Insert initial content if provided
	if content != "" {
		batchPayload, _ := json.Marshal(map[string]interface{}{
			"requests": []map[string]interface{}{
				{
					"insertText": map[string]interface{}{
						"text":                content,
						"endOfSegmentLocation": map[string]interface{}{},
					},
				},
			},
		})

		endpoint := fmt.Sprintf("%s/%s:batchUpdate", docsAPI, doc.DocumentID)
		status, respBody, err := google.DoRequest(flow, "POST", endpoint, token.AccessToken, batchPayload)
		if err != nil || status < 200 || status >= 300 {
			// Document was created but content insertion failed
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			} else {
				errMsg = google.TruncateBody(respBody)
			}
			return map[string]interface{}{
				"tool_result":  fmt.Sprintf("Created document '%s' but failed to insert content: %s", title, errMsg),
				"document_id":  doc.DocumentID,
				"document_url": fmt.Sprintf("https://docs.google.com/document/d/%s/edit", doc.DocumentID),
				"success":      true,
				"error":        errMsg,
			}, nil
		}
	}

	docURL := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", doc.DocumentID)

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Created document '%s' {id:%s}", title, doc.DocumentID),
		"document_id":  doc.DocumentID,
		"document_url": docURL,
		"success":      true,
		"error":        "",
	}, nil
}
