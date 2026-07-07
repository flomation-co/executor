package monday_item_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Item"
	Description  = "Create an item (a row) on a Monday.com board. Optionally place it in a group and set column values."
	Website      = "https://www.flomation.co"
	Icon         = "monday+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to create the item on", Required: true},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "Optionally place the item in this group"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Item Name", Placeholder: "The name of the item", Required: true},
	{Name: "column_values", Type: core.ConnectionTypeObject, Label: "Column Values", Placeholder: "Optional column values as JSON, e.g. {\"status\":{\"label\":\"Done\"}}"},
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
	name, err := monday.RequiredString("name", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	cv, err := monday.ValidateJSON("column_values", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	vars := map[string]interface{}{"boardId": boardID, "itemName": name}
	if g := monday.OptionalString("group_id", inputs); g != "" {
		vars["groupId"] = g
	}
	if cv != "" {
		vars["columnValues"] = cv
	}
	data, err := monday.GraphQL(auth, `mutation ($boardId: ID!, $groupId: String, $itemName: String!, $columnValues: JSON) {
		create_item (board_id: $boardId, group_id: $groupId, item_name: $itemName, column_values: $columnValues) { id name }
	}`, vars)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ResourceResult(monday.ObjectField(data, "create_item"), fmt.Sprintf("Created item %q", name)), nil
}
