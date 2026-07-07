package trello_list_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Lists"
	Description  = "List the lists on a Trello board. Optionally filter by open/closed and narrow the returned Fields."
	Website      = "https://www.flomation.co"
	Icon         = "trello+list"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board whose lists to fetch", Required: true},
	{Name: "filter", Type: core.ConnectionTypeString, Label: "Filter", Placeholder: "Which lists to include", Options: []core.ConnectionOption{
		{Name: "Open", Value: "open"},
		{Name: "Closed", Value: "closed"},
		{Name: "All", Value: "all"},
	}},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return (default: all)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Lists"},
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
	trello.SetIfPresent(params, inputs, "filter", "filter")
	trello.SetFieldsParam(params, inputs, "fields")
	items, err := trello.GetArray(auth, "/boards/"+url.PathEscape(boardID)+"/lists", params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ListResult(items, fmt.Sprintf("Retrieved %d list(s)", len(items))), nil
}
