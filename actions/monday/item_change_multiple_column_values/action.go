package monday_item_change_multiple_column_values

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Change Multiple Column Values"
	Description  = "Set several column values on a Monday.com item at once, as a JSON object of column-id → value."
	Website      = "https://www.flomation.co"
	Icon         = "monday+pen"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board the item is on", Required: true},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Placeholder: "The item to update", Required: true},
	{Name: "column_values", Type: core.ConnectionTypeObject, Label: "Column Values", Placeholder: "JSON of column-id to value, e.g. {\"status\":{\"label\":\"Done\"},\"text\":\"hi\"}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Item ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Item"},
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
	itemID, err := monday.RequiredString("item_id", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	cv, err := monday.ValidateJSON("column_values", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	if cv == "" {
		return monday.ErrorResult("column_values is required"), nil
	}
	data, err := monday.GraphQL(auth, `mutation ($boardId: ID!, $itemId: ID!, $columnValues: JSON!) {
		change_multiple_column_values (board_id: $boardId, item_id: $itemId, column_values: $columnValues) { id }
	}`, map[string]interface{}{"boardId": boardID, "itemId": itemID, "columnValues": cv})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ResourceResult(monday.ObjectField(data, "change_multiple_column_values"), "Updated columns on item "+itemID), nil
}
