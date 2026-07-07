package monday_item_get_by_column_value

import (
	"fmt"

	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Items by Column Value"
	Description  = "Find Monday.com items on a board whose column matches a value."
	Website      = "https://www.flomation.co"
	Icon         = "monday+magnifying-glass"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to search", Required: true},
	{Name: "column_id", Type: core.ConnectionTypeString, Label: "Column", Placeholder: "The column to match on", Required: true},
	{Name: "column_value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "The value to match", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every match (follows pagination)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Items per page (1-100)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Items"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := monday.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	boardID, err := monday.RequiredString("board_id", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	columnID, err := monday.RequiredString("column_id", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	value, err := monday.RequiredString("column_value", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	returnAll := false
	if conn := core.FindConnection("return_all", inputs); conn != nil && conn.Boolean() != nil {
		returnAll = *conn.Boolean()
	}
	limitVal, set := monday.OptionalInt("limit", inputs)
	limit := monday.ClampLimit(limitVal, set)
	data, err := monday.GraphQL(auth, `query ($boardId: ID!, $columnId: String!, $columnValue: String!, $limit: Int) {
		items_page_by_column_values (limit: $limit, board_id: $boardId, columns: [{column_id: $columnId, column_values: [$columnValue]}]) {
			cursor items `+monday.ItemFields+`
		}
	}`, map[string]interface{}{"boardId": boardID, "columnId": columnID, "columnValue": value, "limit": limit})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	page := monday.ObjectField(data, "items_page_by_column_values")
	items, err := monday.CursorItemsAll(auth, page, returnAll)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ListResult(items, fmt.Sprintf("Found %d item(s)", len(items))), nil
}
