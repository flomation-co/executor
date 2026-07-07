package monday_board_archive

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Archive Board"
	Description  = "Archive a Monday.com board by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "monday+box-archive"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to archive", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Board ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Board"},
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
	data, err := monday.GraphQL(auth, `mutation ($id: ID!) { archive_board (board_id: $id) { id } }`,
		map[string]interface{}{"id": boardID})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ResourceResult(monday.ObjectField(data, "archive_board"), "Archived board "+boardID), nil
}
