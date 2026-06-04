// Package replace_text performs find-and-replace in a Google Docs document.
package replace_text

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Replace Text in Document"
	Description  = "Find and replace text in a Google Docs document"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+pencil"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	docsAPI = "https://docs.googleapis.com/v1/documents"
)

var Inputs = [...]core.Connection{
	{Name: "document_id", Type: core.ConnectionTypeString, Label: "Document ID", Required: true},
	{Name: "find", Type: core.ConnectionTypeString, Label: "Find Text", Required: true},
	{Name: "replace", Type: core.ConnectionTypeString, Label: "Replace With", Required: true},
	{Name: "match_case", Type: core.ConnectionTypeBoolean, Label: "Match Case"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "occurrences_replaced", Type: core.ConnectionTypeInteger, Label: "Occurrences Replaced"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	docID := google.OptStr("document_id", inputs)
	if docID == "" {
		return google.ErrorResult("document_id is required")
	}
	find := google.OptStr("find", inputs)
	if find == "" {
		return google.ErrorResult("find text is required")
	}
	replace := google.OptStr("replace", inputs)
	matchCase := google.OptBool("match_case", inputs)

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
				"replaceAllText": map[string]interface{}{
					"containsText": map[string]interface{}{
						"text":      find,
						"matchCase": matchCase,
					},
					"replaceText": replace,
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

	var resp struct {
		Replies []struct {
			ReplaceAllText struct {
				OccurrencesChanged int64 `json:"occurrencesChanged"`
			} `json:"replaceAllText"`
		} `json:"replies"`
	}
	_ = json.Unmarshal(body, &resp)

	var count int64
	if len(resp.Replies) > 0 {
		count = resp.Replies[0].ReplaceAllText.OccurrencesChanged
	}

	return map[string]interface{}{
		"tool_result":           fmt.Sprintf("Replaced %d occurrence(s) of '%s' with '%s'", count, find, replace),
		"occurrences_replaced":  count,
		"success":               true,
		"error":                 "",
	}, nil
}
