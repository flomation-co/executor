// Package create_sheet adds a new sheet (tab) to an existing spreadsheet.
package create_sheet

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Add Sheet Tab"
	Description  = "Add a new sheet tab to a Google Sheets spreadsheet"
	Website      = "https://www.flomation.co"
	Icon         = "table+plus"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	sheetsAPI = "https://sheets.googleapis.com/v4/spreadsheets"
)

var Inputs = [...]core.Connection{
	{Name: "spreadsheet_id", Type: core.ConnectionTypeString, Label: "Spreadsheet ID", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Sheet Name", Required: true, Placeholder: "Data"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "sheet_id", Type: core.ConnectionTypeInteger, Label: "Sheet ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	spreadsheetID := google.OptStr("spreadsheet_id", inputs)
	if spreadsheetID == "" {
		return google.ErrorResult("spreadsheet_id is required")
	}
	title := google.OptStr("title", inputs)
	if title == "" {
		return google.ErrorResult("title is required")
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

	payload := map[string]interface{}{
		"requests": []map[string]interface{}{
			{
				"addSheet": map[string]interface{}{
					"properties": map[string]interface{}{
						"title": title,
					},
				},
			},
		},
	}
	body, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("%s/%s:batchUpdate", sheetsAPI, spreadsheetID)

	status, respBody, err := google.DoRequest(flow, "POST", endpoint, token.AccessToken, body)
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
		Replies []struct {
			AddSheet struct {
				Properties struct {
					SheetID int64  `json:"sheetId"`
					Title   string `json:"title"`
				} `json:"properties"`
			} `json:"addSheet"`
		} `json:"replies"`
	}
	_ = json.Unmarshal(respBody, &resp)

	var sheetID int64
	if len(resp.Replies) > 0 {
		sheetID = resp.Replies[0].AddSheet.Properties.SheetID
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Added sheet '%s' (ID: %d)", title, sheetID),
		"sheet_id":    sheetID,
		"success":     true,
		"error":       "",
	}, nil
}
