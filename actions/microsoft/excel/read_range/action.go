// Package read_range reads a cell range from a Microsoft Excel Online workbook.
package read_range

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Read Range"
	Description  = "Read a cell range from a Microsoft Excel Online workbook"
	Website      = "https://www.flomation.co"
	Icon         = "table+eye"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Workbook Item ID", Required: true},
	{Name: "sheet", Type: core.ConnectionTypeString, Label: "Worksheet Name", Required: true, Placeholder: "Sheet1"},
	{Name: "range_address", Type: core.ConnectionTypeString, Label: "Cell Range", Required: true, Placeholder: "A1:D10"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_EXCEL}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "values", Type: core.ConnectionTypeString, Label: "Values (JSON 2D array)"},
	{Name: "row_count", Type: core.ConnectionTypeString, Label: "Row Count"},
	{Name: "column_count", Type: core.ConnectionTypeString, Label: "Column Count"},
	{Name: "data", Type: core.ConnectionTypeString, Label: "Full Response (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	sheet := microsoft.OptStr("sheet", inputs)
	rangeAddr := microsoft.OptStr("range_address", inputs)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	if itemID == "" {
		return microsoft.ErrorResult("workbook item ID is required")
	}
	if sheet == "" {
		return microsoft.ErrorResult("worksheet name is required")
	}
	if rangeAddr == "" {
		return microsoft.ErrorResult("cell range is required")
	}

	tokens, err := microsoft.FetchTokens(flow, credential, "excel")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	endpoint := fmt.Sprintf("%s/me/drive/items/%s/workbook/worksheets/%s/range(address='%s')",
		microsoft.GraphAPI, itemID, sheet, rangeAddr)

	status, body, err := microsoft.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(body)))
	}

	var resp struct {
		Values      [][]interface{} `json:"values"`
		RowCount    int             `json:"rowCount"`
		ColumnCount int             `json:"columnCount"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return microsoft.ErrorResult("failed to parse response: " + err.Error())
	}

	valuesJSON, _ := json.Marshal(resp.Values)

	// Format as a text table for tool_result
	var lines []string
	for _, row := range resp.Values {
		var cells []string
		for _, cell := range row {
			cells = append(cells, fmt.Sprintf("%v", cell))
		}
		lines = append(lines, strings.Join(cells, "\t"))
	}
	toolResult := fmt.Sprintf("Read %dx%d cells from %s:\n%s",
		resp.RowCount, resp.ColumnCount, rangeAddr, strings.Join(lines, "\n"))

	return map[string]interface{}{
		"tool_result":  toolResult,
		"values":       string(valuesJSON),
		"row_count":    fmt.Sprintf("%d", resp.RowCount),
		"column_count": fmt.Sprintf("%d", resp.ColumnCount),
		"data":         string(body),
		"success":      true,
		"error":        "",
	}, nil
}
