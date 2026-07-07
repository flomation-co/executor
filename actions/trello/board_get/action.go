package trello_board_get

import (
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Board"
	Description  = "Fetch a single Trello board by its ID. Optionally narrow the returned data with a comma-separated Fields list."
	Website      = "https://www.flomation.co"
	Icon         = "trello+magnifying-glass"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board to fetch", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. name,desc,url (default: all)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Board ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Board"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := trello.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := trello.RequiredString("id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	trello.SetFieldsParam(params, inputs, "fields")
	obj, err := trello.GetObject(auth, "/boards/"+url.PathEscape(id), params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, "Retrieved board "+id), nil
}
