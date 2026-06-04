// Package append_text appends text to the end of a Google Docs document.
package append_text

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Append to Document"
	Description  = "Append text to the end of a Google Docs document"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+plus"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	docsAPI = "https://docs.googleapis.com/v1/documents"
)

var Inputs = [...]core.Connection{
	{Name: "document_id", Type: core.ConnectionTypeString, Label: "Document ID", Required: true},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Text to Append", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	docID := google.OptStr("document_id", inputs)
	if docID == "" {
		return google.ErrorResult("document_id is required")
	}
	text := google.OptStr("text", inputs)
	if text == "" {
		return google.ErrorResult("text is required")
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

	payload, _ := json.Marshal(map[string]interface{}{
		"requests": []map[string]interface{}{
			{
				"insertText": map[string]interface{}{
					"text":                text,
					"endOfSegmentLocation": map[string]interface{}{},
				},
			},
		},
	})

	endpoint := fmt.Sprintf("%s/%s:batchUpdate", docsAPI, docID)

	status, body, err := google.DoRequest(flow, "POST", endpoint, token.AccessToken, payload)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Appended %d characters to document %s", len(text), docID),
		"success":     true,
		"error":       "",
	}, nil
}
