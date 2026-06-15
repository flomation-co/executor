// Package read reads the content of a Google Docs document.
package read

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Read Document"
	Description  = "Read the content of a Google Docs document"
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
	{Name: "content", Type: core.ConnectionTypeString, Label: "Document Text"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "document", Type: core.ConnectionTypeString, Label: "Full Document (JSON)"},
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

	endpoint := fmt.Sprintf("%s/%s", docsAPI, docID)

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
		Title string `json:"title"`
		Body  struct {
			Content []struct {
				Paragraph *struct {
					Elements []struct {
						TextRun *struct {
							Content string `json:"content"`
						} `json:"textRun,omitempty"`
					} `json:"elements"`
				} `json:"paragraph,omitempty"`
			} `json:"content"`
		} `json:"body"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse document: %v", err))
	}

	// Extract plain text
	var textBuilder strings.Builder
	for _, content := range doc.Body.Content {
		if content.Paragraph != nil {
			for _, elem := range content.Paragraph.Elements {
				if elem.TextRun != nil {
					textBuilder.WriteString(elem.TextRun.Content)
				}
			}
		}
	}
	text := strings.TrimSpace(textBuilder.String())

	return map[string]interface{}{
		"tool_result": text,
		"content":     text,
		"title":       doc.Title,
		"document":    string(body),
		"success":     true,
		"error":       "",
	}, nil
}
