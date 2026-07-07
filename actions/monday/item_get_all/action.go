package monday_item_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Items"
	Description  = "List the items in a Monday.com group on a board."
	Website      = "https://www.flomation.co"
	Icon         = "monday+list"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to list items from", Required: true},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group", Placeholder: "The group whose items to list", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every item (follows pagination)"},
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
	groupID, err := monday.RequiredString("group_id", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	returnAll := false
	if conn := core.FindConnection("return_all", inputs); conn != nil && conn.Boolean() != nil {
		returnAll = *conn.Boolean()
	}
	limitVal, set := monday.OptionalInt("limit", inputs)
	limit := monday.ClampLimit(limitVal, set)
	data, err := monday.GraphQL(auth, `query ($boardId: [ID!], $groupId: [String], $limit: Int) {
		boards (ids: $boardId) { groups (ids: $groupId) { items_page (limit: $limit) { cursor items `+monday.ItemFields+` } } }
	}`, map[string]interface{}{"boardId": []string{boardID}, "groupId": []string{groupID}, "limit": limit})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	board := monday.FirstBoard(data)
	if board == nil {
		return monday.ErrorResult("board " + boardID + " not found"), nil
	}
	groups := monday.ArrayField(board, "groups")
	if len(groups) == 0 {
		return monday.ListResult(nil, "Retrieved 0 item(s)"), nil
	}
	g, _ := groups[0].(map[string]interface{})
	page := monday.ObjectField(g, "items_page")
	items, err := monday.CursorItemsAll(auth, page, returnAll)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ListResult(items, fmt.Sprintf("Retrieved %d item(s)", len(items))), nil
}
