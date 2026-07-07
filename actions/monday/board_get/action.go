package monday_board_get

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Board"
	Description  = "Fetch a single Monday.com board by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "monday+magnifying-glass"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to fetch", Required: true},
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
	data, err := monday.GraphQL(auth, `query ($id: [ID!]) { boards (ids: $id) { `+monday.BoardFields+` } }`,
		map[string]interface{}{"id": []string{boardID}})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	board := monday.FirstBoard(data)
	if board == nil {
		return monday.ErrorResult("board " + boardID + " not found"), nil
	}
	return monday.ResourceResult(board, "Retrieved board "+boardID), nil
}
