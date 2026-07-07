package monday_item_move

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Move Item to Group"
	Description  = "Move a Monday.com item into a different group on its board."
	Website      = "https://www.flomation.co"
	Icon         = "monday+arrow-right-arrow-left"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board the item is on (used to load the group picker)"},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Placeholder: "The item to move", Required: true},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "The group to move the item into", Required: true},
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
	itemID, err := monday.RequiredString("item_id", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	groupID, err := monday.RequiredString("group_id", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	data, err := monday.GraphQL(auth, `mutation ($groupId: String!, $itemId: ID!) { move_item_to_group (group_id: $groupId, item_id: $itemId) { id } }`,
		map[string]interface{}{"groupId": groupID, "itemId": itemID})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ResourceResult(monday.ObjectField(data, "move_item_to_group"), "Moved item "+itemID+" to group "+groupID), nil
}
