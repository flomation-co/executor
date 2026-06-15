// Package write_range writes values to a cell range in a Microsoft Excel Online workbook.
package write_range

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Write Range"
	Description  = "Write values to a cell range in a Microsoft Excel Online workbook"
	Website      = "https://www.flomation.co"
	Icon         = "table+pencil"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Workbook Item ID", Required: true},
	{Name: "sheet", Type: core.ConnectionTypeString, Label: "Worksheet Name", Required: true, Placeholder: "Sheet1"},
	{Name: "range_address", Type: core.ConnectionTypeString, Label: "Cell Range", Required: true, Placeholder: "A1:D3"},
	{Name: "values", Type: core.ConnectionTypeText, Label: "Values (JSON 2D array)", Required: true, Placeholder: "[[\"Name\",\"Age\"],[\"Alice\",30]]"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_EXCEL}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	sheet := microsoft.OptStr("sheet", inputs)
	rangeAddr := microsoft.OptStr("range_address", inputs)
	valuesStr := microsoft.OptStr("values", inputs)
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
	if valuesStr == "" {
		return microsoft.ErrorResult("values are required")
	}

	// Parse the 2D array from JSON
	var values [][]interface{}
	if err := json.Unmarshal([]byte(valuesStr), &values); err != nil {
		return microsoft.ErrorResult("failed to parse values JSON: " + err.Error())
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

	payload := map[string]interface{}{
		"values": values,
	}
	body, _ := json.Marshal(payload)

	status, respBody, err := microsoft.DoRequest(flow, "PATCH", endpoint, token.AccessToken, body)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(respBody)))
	}

	rows := len(values)
	cols := 0
	if rows > 0 {
		cols = len(values[0])
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Wrote %dx%d cells to %s", rows, cols, rangeAddr),
		"success":     true,
		"error":       "",
	}, nil
}
