package monday_board_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Boards"
	Description  = "List the Monday.com boards you can access."
	Website      = "https://www.flomation.co"
	Icon         = "monday+list"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every board (ignores Limit)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum number of boards to return (1-100)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Boards"},
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
	returnAll := false
	if conn := core.FindConnection("return_all", inputs); conn != nil && conn.Boolean() != nil {
		returnAll = *conn.Boolean()
	}
	limitVal, set := monday.OptionalInt("limit", inputs)
	limit := monday.ClampLimit(limitVal, set)
	all := []interface{}{}
	page := 1
	for {
		data, err := monday.GraphQL(auth, `query ($page: Int, $limit: Int) { boards (page: $page, limit: $limit) { `+monday.BoardFields+` } }`,
			map[string]interface{}{"page": page, "limit": limit})
		if err != nil {
			return monday.ErrorResult(err.Error()), nil
		}
		boards := monday.ArrayField(data, "boards")
		all = append(all, boards...)
		if !returnAll || len(boards) < limit || page >= monday.MaxAllPages {
			break
		}
		page++
	}
	return monday.ListResult(all, fmt.Sprintf("Retrieved %d board(s)", len(all))), nil
}
