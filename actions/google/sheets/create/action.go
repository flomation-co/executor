// Package create creates a new Google Sheets spreadsheet.
package create

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
	Name         = "Create Spreadsheet"
	Description  = "Create a new Google Sheets spreadsheet"
	Website      = "https://www.flomation.co"
	Icon         = "table+plus"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	sheetsAPI = "https://sheets.googleapis.com/v4/spreadsheets"
)

var Inputs = [...]core.Connection{
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Required: true, Placeholder: "My Spreadsheet"},
	{Name: "sheet_names", Type: core.ConnectionTypeString, Label: "Sheet Names (comma-separated)", Placeholder: "Data, Summary"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "spreadsheet_id", Type: core.ConnectionTypeString, Label: "Spreadsheet ID"},
	{Name: "spreadsheet_url", Type: core.ConnectionTypeString, Label: "Spreadsheet URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	title := google.OptStr("title", inputs)
	if title == "" {
		return google.ErrorResult("title is required")
	}
	sheetNames := google.OptStr("sheet_names", inputs)

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

	payload := map[string]interface{}{
		"properties": map[string]interface{}{
			"title": title,
		},
	}

	// Add custom sheet tabs if specified
	if sheetNames != "" {
		var sheets []map[string]interface{}
		for _, name := range strings.Split(sheetNames, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				sheets = append(sheets, map[string]interface{}{
					"properties": map[string]interface{}{
						"title": name,
					},
				})
			}
		}
		if len(sheets) > 0 {
			payload["sheets"] = sheets
		}
	}

	body, _ := json.Marshal(payload)

	status, respBody, err := google.DoRequest(flow, "POST", sheetsAPI, token.AccessToken, body)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(respBody)))
	}

	var resp struct {
		SpreadsheetID  string `json:"spreadsheetId"`
		SpreadsheetURL string `json:"spreadsheetUrl"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created spreadsheet '%s' {id:%s}", title, resp.SpreadsheetID),
		"spreadsheet_id":  resp.SpreadsheetID,
		"spreadsheet_url": resp.SpreadsheetURL,
		"success":         true,
		"error":           "",
	}, nil
}
