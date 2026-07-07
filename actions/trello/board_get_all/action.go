package trello_board_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Boards"
	Description  = "List the Trello boards you can access. Optionally filter (e.g. open, closed, starred) and narrow the returned Fields."
	Website      = "https://www.flomation.co"
	Icon         = "trello+list"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "filter", Type: core.ConnectionTypeString, Label: "Filter", Placeholder: "Which boards to include", Options: []core.ConnectionOption{
		{Name: "All", Value: "all"},
		{Name: "Open", Value: "open"},
		{Name: "Closed", Value: "closed"},
		{Name: "Starred", Value: "starred"},
	}},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. name,url (default: all)"},
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
	auth, err := trello.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	trello.SetIfPresent(params, inputs, "filter", "filter")
	trello.SetFieldsParam(params, inputs, "fields")
	items, err := trello.GetArray(auth, "/members/me/boards", params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ListResult(items, fmt.Sprintf("Retrieved %d board(s)", len(items))), nil
}
