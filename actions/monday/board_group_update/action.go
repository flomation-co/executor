package monday_board_group_update

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Group"
	Description  = "Rename a Monday.com group or change its colour."
	Website      = "https://www.flomation.co"
	Icon         = "monday+pen"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board the group belongs to", Required: true},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "The group to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A new name for the group"},
	{Name: "color", Type: core.ConnectionTypeString, Label: "Color", Placeholder: "A new colour for the group (e.g. #037f4c)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Group"},
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
	name := monday.OptionalString("name", inputs)
	color := monday.OptionalString("color", inputs)
	if name == "" && color == "" {
		return monday.ErrorResult("provide a new Name or Color to update the group"), nil
	}
	// update_group changes ONE attribute per call, so send one mutation per
	// provided attribute (title for a rename, color for a recolour).
	query := `mutation ($boardId: ID!, $groupId: String!, $attr: GroupAttributes!, $val: String!) {
		update_group (board_id: $boardId, group_id: $groupId, group_attribute: $attr, new_value: $val) { id title color }
	}`
	var last map[string]interface{}
	if name != "" {
		data, err := monday.GraphQL(auth, query, map[string]interface{}{"boardId": boardID, "groupId": groupID, "attr": "title", "val": name})
		if err != nil {
			return monday.ErrorResult(err.Error()), nil
		}
		last = monday.ObjectField(data, "update_group")
	}
	if color != "" {
		data, err := monday.GraphQL(auth, query, map[string]interface{}{"boardId": boardID, "groupId": groupID, "attr": "color", "val": color})
		if err != nil {
			return monday.ErrorResult(err.Error()), nil
		}
		last = monday.ObjectField(data, "update_group")
	}
	return monday.ResourceResult(last, "Updated group "+groupID), nil
}
