// Package delete_sheet removes a sheet (tab) from a spreadsheet.
package delete_sheet

import (
	"encoding/json"
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Delete Sheet Tab"
	Description  = "Delete a sheet tab from a Google Sheets spreadsheet"
	Website      = "https://www.flomation.co"
	Icon         = "table"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	sheetsAPI = "https://sheets.googleapis.com/v4/spreadsheets"
)

var Inputs = [...]core.Connection{
	{Name: "spreadsheet_id", Type: core.ConnectionTypeString, Label: "Spreadsheet ID", Required: true},
	{Name: "sheet_id", Type: core.ConnectionTypeInteger, Label: "Sheet ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	spreadsheetID := google.OptStr("spreadsheet_id", inputs)
	if spreadsheetID == "" {
		return google.ErrorResult("spreadsheet_id is required")
	}

	sheetIDStr := google.OptStr("sheet_id", inputs)
	if sheetIDStr == "" {
		return google.ErrorResult("sheet_id is required")
	}
	sheetID, err := strconv.ParseInt(sheetIDStr, 10, 64)
	if err != nil {
		return google.ErrorResult(fmt.Sprintf("invalid sheet_id: %v", err))
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
				"deleteSheet": map[string]interface{}{
					"sheetId": sheetID,
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

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted sheet %d from spreadsheet %s", sheetID, spreadsheetID),
		"success":     true,
		"error":       "",
	}, nil
}
