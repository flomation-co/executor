package monday_board_group_delete

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Delete Group"
	Description  = "Delete a group from a Monday.com board."
	Website      = "https://www.flomation.co"
	Icon         = "monday+trash"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board the group belongs to", Required: true},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "The group to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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
	groupID, err := monday.RequiredString("group_id", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	data, err := monday.GraphQL(auth, `mutation ($boardId: ID!, $groupId: String!) { delete_group (board_id: $boardId, group_id: $groupId) { id } }`,
		map[string]interface{}{"boardId": boardID, "groupId": groupID})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	_ = data
	return monday.SuccessResult(groupID, nil, "Deleted group "+groupID), nil
}
