// Package append appends rows to a Google Sheet.
package append

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Append to Sheet"
	Description  = "Append rows to a Google Sheet"
	Website      = "https://www.flomation.co"
	Icon         = "table+plus"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	sheetsAPI = "https://sheets.googleapis.com/v4/spreadsheets"
)

var Inputs = [...]core.Connection{
	{Name: "spreadsheet_id", Type: core.ConnectionTypeString, Label: "Spreadsheet ID", Required: true},
	{Name: "range", Type: core.ConnectionTypeString, Label: "Sheet/Range", Required: true, Placeholder: "Sheet1"},
	{Name: "data", Type: core.ConnectionTypeRows, Label: "Rows to append", Required: true, Placeholder: `[["Alice",30],["Bob",25]]`},
	{
		Name:  "value_input",
		Type:  core.ConnectionTypeString,
		Label: "Value Input Option",
		Options: []core.ConnectionOption{
			{Name: "User Entered (auto-format)", Value: "USER_ENTERED"},
			{Name: "Raw (as-is)", Value: "RAW"},
		},
	},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "updated_range", Type: core.ConnectionTypeString, Label: "Updated Range"},
	{Name: "updated_rows", Type: core.ConnectionTypeInteger, Label: "Rows Appended"},
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
	dataStr := google.OptStr("data", inputs)
	if dataStr == "" {
		return google.ErrorResult("data is required")
	}
	valueInput := google.OptStr("value_input", inputs)
	if valueInput == "" {
		valueInput = "USER_ENTERED"
	}

	credential := google.OptStr("credential", inputs)
	account := google.OptStr("account", inputs)

	var values [][]interface{}
	if err := json.Unmarshal([]byte(dataStr), &values); err != nil {
		return google.ErrorResult(fmt.Sprintf("data must be a JSON 2D array: %v", err))
	}

	// Under USER_ENTERED, Sheets numerically coerces phone/ID-shaped
	// strings (dropping leading zeros, evaluating a leading +). Guard
	// them so they land as text. RAW already stores values verbatim.
	if valueInput == "USER_ENTERED" {
		google.ProtectPhoneLikeText(values)
	}

	tokens, err := google.FetchTokens(flow, credential)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	active := google.FilterTokens(tokens, account)
	if len(active) == 0 {
		return google.ErrorResult("no active Google account available")
	}
	token := active[0]

	// Anchor a bare sheet-name range to A1 so Google's append aligns new
	// rows to column A instead of a mis-detected far-column table (e.g.
	// appending at L2 rather than A2). See google.AnchorAppendRange.
	searchRange := google.AnchorAppendRange(cellRange)

	payload := map[string]interface{}{
		"range":  searchRange,
		"values": values,
	}
	body, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("%s/%s/values/%s:append?valueInputOption=%s&insertDataOption=INSERT_ROWS",
		sheetsAPI, spreadsheetID, searchRange, valueInput)

	status, respBody, err := google.DoRequest(flow, "POST", endpoint, token.AccessToken, body)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status == 401 || status == 403 {
		google.HandleAuthError(flow, token.Email, status)
		return google.ErrorResult(fmt.Sprintf("Google API returned %d", status))
	}
	if status != 200 {
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(respBody)))
	}

	var resp struct {
		Updates struct {
			UpdatedRange string `json:"updatedRange"`
			UpdatedRows  int64  `json:"updatedRows"`
		} `json:"updates"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Appended %d row(s) to %s", resp.Updates.UpdatedRows, resp.Updates.UpdatedRange),
		"updated_range": resp.Updates.UpdatedRange,
		"updated_rows":  resp.Updates.UpdatedRows,
		"success":       true,
		"error":         "",
	}, nil
}
