package monday_board_column_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Columns"
	Description  = "List the columns of a Monday.com board."
	Website      = "https://www.flomation.co"
	Icon         = "monday+list"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board whose columns to fetch", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Columns"},
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
	data, err := monday.GraphQL(auth, `query ($boardId: [ID!]) { boards (ids: $boardId) { columns { id title type settings_str archived } } }`,
		map[string]interface{}{"boardId": []string{boardID}})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	board := monday.FirstBoard(data)
	if board == nil {
		return monday.ErrorResult("board " + boardID + " not found"), nil
	}
	cols := monday.ArrayField(board, "columns")
	return monday.ListResult(cols, fmt.Sprintf("Retrieved %d column(s)", len(cols))), nil
}
