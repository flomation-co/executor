// Package clear clears a range of cells in a Google Sheet.
package clear

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Clear Sheet Range"
	Description  = "Clear a range of cells in a Google Sheet"
	Website      = "https://www.flomation.co"
	Icon         = "table"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	sheetsAPI = "https://sheets.googleapis.com/v4/spreadsheets"
)

var Inputs = [...]core.Connection{
	{Name: "spreadsheet_id", Type: core.ConnectionTypeString, Label: "Spreadsheet ID", Required: true},
	{Name: "range", Type: core.ConnectionTypeString, Label: "Range", Required: true, Placeholder: "Sheet1!A1:D10"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "cleared_range", Type: core.ConnectionTypeString, Label: "Cleared Range"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	spreadsheetID := google.OptStr("spreadsheet_id", inputs)
	if spreadsheetID == "" {
		return google.ErrorResult("spreadsheet_id is required")
	}
	cellRange := google.OptStr("range", inputs)
	if cellRange == "" {
		return google.ErrorResult("range is required")
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

	endpoint := fmt.Sprintf("%s/%s/values/%s:clear", sheetsAPI, spreadsheetID, cellRange)

	status, body, err := google.DoRequest(flow, "POST", endpoint, token.AccessToken, []byte("{}"))
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
		ClearedRange string `json:"clearedRange"`
	}
	_ = json.Unmarshal(body, &resp)

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Cleared %s", resp.ClearedRange),
		"cleared_range": resp.ClearedRange,
		"success":       true,
		"error":         "",
	}, nil
}
