// Package read reads a range of cells from a Google Sheets spreadsheet.
package read

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Read Sheet"
	Description  = "Read a range of cells from a Google Sheet"
	Website      = "https://www.flomation.co"
	Icon         = "table+eye"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	sheetsAPI = "https://sheets.googleapis.com/v4/spreadsheets"
)

var Inputs = [...]core.Connection{
	{Name: "spreadsheet_id", Type: core.ConnectionTypeString, Label: "Spreadsheet ID", Required: true},
	{Name: "range", Type: core.ConnectionTypeString, Label: "Range", Required: true, Placeholder: "Sheet1!A1:D10"},
	{
		Name:  "value_render",
		Type:  core.ConnectionTypeString,
		Label: "Value Render Option",
		Options: []core.ConnectionOption{
			{Name: "Formatted", Value: "FORMATTED_VALUE"},
			{Name: "Unformatted", Value: "UNFORMATTED_VALUE"},
			{Name: "Formula", Value: "FORMULA"},
		},
	},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "data", Type: core.ConnectionTypeString, Label: "Data (JSON 2D array)"},
	{Name: "rows", Type: core.ConnectionTypeInteger, Label: "Row Count"},
	{Name: "columns", Type: core.ConnectionTypeInteger, Label: "Column Count"},
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
	valueRender := google.OptStr("value_render", inputs)
	if valueRender == "" {
		valueRender = "FORMATTED_VALUE"
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

	endpoint := fmt.Sprintf("%s/%s/values/%s?valueRenderOption=%s",
		sheetsAPI, spreadsheetID, cellRange, valueRender)

	status, body, err := google.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status == 401 || status == 403 {
		google.HandleAuthError(flow, token.Email, status)
		return google.ErrorResult(fmt.Sprintf("Google API returned %d", status))
	}
	if status == 404 {
		return google.ErrorResult(fmt.Sprintf("Spreadsheet not found: %s", spreadsheetID))
	}
	if status != 200 {
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	var resp struct {
		Range  string          `json:"range"`
		Values [][]interface{} `json:"values"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	rows := int64(len(resp.Values))
	var cols int64
	for _, row := range resp.Values {
		if int64(len(row)) > cols {
			cols = int64(len(row))
		}
	}

	dataJSON, _ := json.Marshal(resp.Values)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Read %d rows x %d columns from %s", rows, cols, resp.Range),
		"data":        string(dataJSON),
		"rows":        rows,
		"columns":     cols,
		"success":     true,
		"error":       "",
	}, nil
}
