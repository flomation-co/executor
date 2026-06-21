// Package get_metadata retrieves metadata for a Google Sheets spreadsheet.
package get_metadata

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Sheet Metadata"
	Description  = "Get metadata for a Google Sheets spreadsheet"
	Website      = "https://www.flomation.co"
	Icon         = "table+eye"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	sheetsAPI = "https://sheets.googleapis.com/v4/spreadsheets"
)

var Inputs = [...]core.Connection{
	{Name: "spreadsheet_id", Type: core.ConnectionTypeString, Label: "Spreadsheet ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "sheets", Type: core.ConnectionTypeString, Label: "Sheets (JSON)"},
	{Name: "sheet_count", Type: core.ConnectionTypeInteger, Label: "Sheet Count"},
	{Name: "spreadsheet", Type: core.ConnectionTypeString, Label: "Full Metadata (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	spreadsheetID := google.OptStr("spreadsheet_id", inputs)
	if spreadsheetID == "" {
		return google.ErrorResult("spreadsheet_id is required")
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

	endpoint := fmt.Sprintf("%s/%s?fields=spreadsheetId,properties.title,sheets(properties(sheetId,title,index,gridProperties))",
		sheetsAPI, spreadsheetID)

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

	var resp struct {
		Properties struct {
			Title string `json:"title"`
		} `json:"properties"`
		Sheets []struct {
			Properties struct {
				SheetID        int64  `json:"sheetId"`
				Title          string `json:"title"`
				Index          int64  `json:"index"`
				GridProperties struct {
					RowCount    int64 `json:"rowCount"`
					ColumnCount int64 `json:"columnCount"`
				} `json:"gridProperties"`
			} `json:"properties"`
		} `json:"sheets"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	sheetsJSON, _ := json.Marshal(resp.Sheets)

	var summary string
	summary = fmt.Sprintf("'%s' — %d sheet(s):", resp.Properties.Title, len(resp.Sheets))
	for _, s := range resp.Sheets {
		summary += fmt.Sprintf("\n  - %s (%dx%d)", s.Properties.Title,
			s.Properties.GridProperties.RowCount, s.Properties.GridProperties.ColumnCount)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"title":       resp.Properties.Title,
		"sheets":      string(sheetsJSON),
		"sheet_count": int64(len(resp.Sheets)),
		"spreadsheet": string(body),
		"success":     true,
		"error":       "",
	}, nil
}
