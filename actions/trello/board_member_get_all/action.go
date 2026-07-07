package trello_board_member_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Board Members"
	Description  = "List the members of a Trello board. Leave Return All on (the default behaviour when no Limit is set) to get every member; set a Limit and turn Return All off to cap how many are returned."
	Website      = "https://www.flomation.co"
	Icon         = "trello+user"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board whose members to list", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every member (ignores Limit)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum number of members to return"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Members"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := trello.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	boardID, err := trello.RequiredString("board_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	returnAll, _ := trello.OptionalBoolSet("return_all", inputs)
	if !returnAll {
		if err := trello.SetIntIfPresent(params, inputs, "limit", "limit"); err != nil {
			return trello.ErrorResult(err.Error()), nil
		}
	}
	items, err := trello.GetArray(auth, "/boards/"+url.PathEscape(boardID)+"/members", params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ListResult(items, fmt.Sprintf("Retrieved %d member(s)", len(items))), nil
}
