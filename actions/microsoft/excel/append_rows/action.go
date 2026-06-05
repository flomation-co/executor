// Package append_rows appends rows to a table in a Microsoft Excel Online workbook.
package append_rows

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Append Rows"
	Description  = "Append rows to a table in a Microsoft Excel Online workbook"
	Website      = "https://www.flomation.co"
	Icon         = "table+plus"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Workbook Item ID", Required: true},
	{Name: "table_name", Type: core.ConnectionTypeString, Label: "Table Name", Required: true, Placeholder: "Table1"},
	{Name: "values", Type: core.ConnectionTypeText, Label: "Row Values (JSON 2D array)", Required: true, Placeholder: "[[\"Alice\",30],[\"Bob\",25]]"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_EXCEL}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "row_index", Type: core.ConnectionTypeString, Label: "Row Index"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	tableName := microsoft.OptStr("table_name", inputs)
	valuesStr := microsoft.OptStr("values", inputs)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	if itemID == "" {
		return microsoft.ErrorResult("workbook item ID is required")
	}
	if tableName == "" {
		return microsoft.ErrorResult("table name is required")
	}
	if valuesStr == "" {
		return microsoft.ErrorResult("row values are required")
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

	endpoint := fmt.Sprintf("%s/me/drive/items/%s/workbook/tables/%s/rows/add",
		microsoft.GraphAPI, itemID, tableName)

	payload := map[string]interface{}{
		"values": values,
	}
	body, _ := json.Marshal(payload)

	status, respBody, err := microsoft.DoRequest(flow, "POST", endpoint, token.AccessToken, body)
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			microsoft.HandleAuthError(flow, token.Email, status)
		}
		return microsoft.ErrorResult(fmt.Sprintf("Graph API returned %d: %s", status, microsoft.TruncateBody(respBody)))
	}

	var resp struct {
		Index int `json:"index"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return microsoft.ErrorResult("failed to parse response: " + err.Error())
	}

	rowCount := len(values)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Appended %d rows to %s", rowCount, tableName),
		"row_index":   fmt.Sprintf("%d", resp.Index),
		"success":     true,
		"error":       "",
	}, nil
}
