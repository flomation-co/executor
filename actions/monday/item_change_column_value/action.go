package monday_item_change_column_value

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Change Column Value"
	Description  = "Set a single column's value on a Monday.com item. The value is column-type-specific JSON."
	Website      = "https://www.flomation.co"
	Icon         = "monday+pen"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board the item is on", Required: true},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Placeholder: "The item to update", Required: true},
	{Name: "column_id", Type: core.ConnectionTypeString, Label: "Column", Placeholder: "The column to set", Required: true},
	{Name: "value", Type: core.ConnectionTypeObject, Label: "Value", Placeholder: "The new value as JSON, e.g. {\"label\":\"Done\"} for a status", Required: true},
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
	columnID, err := monday.RequiredString("column_id", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	value, err := monday.ValidateJSON("value", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	if value == "" {
		return monday.ErrorResult("value is required"), nil
	}
	data, err := monday.GraphQL(auth, `mutation ($boardId: ID!, $itemId: ID!, $columnId: String!, $value: JSON!) {
		change_column_value (board_id: $boardId, item_id: $itemId, column_id: $columnId, value: $value) { id }
	}`, map[string]interface{}{"boardId": boardID, "itemId": itemID, "columnId": columnID, "value": value})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ResourceResult(monday.ObjectField(data, "change_column_value"), "Updated column "+columnID+" on item "+itemID), nil
}
