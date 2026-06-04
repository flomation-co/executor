// Package list_worksheets lists worksheets in a Microsoft Excel Online workbook.
package list_worksheets

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
	Name         = "List Worksheets"
	Description  = "List all worksheets in a Microsoft Excel Online workbook"
	Website      = "https://www.flomation.co"
	Icon         = "table+list"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Workbook Item ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_EXCEL}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "worksheets", Type: core.ConnectionTypeString, Label: "Worksheets (JSON)"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Worksheet Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	if itemID == "" {
		return microsoft.ErrorResult("workbook item ID is required")
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

	endpoint := fmt.Sprintf("%s/me/drive/items/%s/workbook/worksheets",
		microsoft.GraphAPI, itemID)

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
		Value []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Position   int    `json:"position"`
			Visibility string `json:"visibility"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return microsoft.ErrorResult("failed to parse response: " + err.Error())
	}

	worksheetsJSON, _ := json.Marshal(resp.Value)
	count := len(resp.Value)

	var names []string
	for _, ws := range resp.Value {
		names = append(names, ws.Name)
	}

	toolResult := fmt.Sprintf("Found %d worksheet(s): %s", count, strings.Join(names, ", "))
	if count == 0 {
		toolResult = "No worksheets found"
	}

	return map[string]interface{}{
		"tool_result": toolResult,
		"worksheets":  string(worksheetsJSON),
		"count":       fmt.Sprintf("%d", count),
		"success":     true,
		"error":       "",
	}, nil
}
