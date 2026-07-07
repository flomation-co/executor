package trello_list_update

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update List"
	Description  = "Change an existing Trello list — rename it, archive/reopen it, move it to another board, or reposition it."
	Website      = "https://www.flomation.co"
	Icon         = "trello+pen"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board that owns the list (used to load the list picker)"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "List", Placeholder: "The list to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A new name for the list"},
	{Name: "closed", Type: core.ConnectionTypeBoolean, Label: "Closed", Placeholder: "Archive the list, or reopen it"},
	{Name: "move_to_board_id", Type: core.ConnectionTypeString, Label: "Move to Board", Placeholder: "Move the list to this board ID"},
	{Name: "position", Type: core.ConnectionTypeString, Label: "Position", Placeholder: "top, bottom, or a positive number"},
	{Name: "subscribed", Type: core.ConnectionTypeBoolean, Label: "Subscribed", Placeholder: "Subscribe to the list"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "List ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "List"},
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
	trello.SetIfPresent(params, inputs, "name", "name")
	trello.SetBoolIfSet(params, inputs, "closed", "closed")
	trello.SetIfPresent(params, inputs, "idBoard", "move_to_board_id")
	trello.SetIfPresent(params, inputs, "pos", "position")
	trello.SetBoolIfSet(params, inputs, "subscribed", "subscribed")
	obj, err := trello.WriteObject(auth, http.MethodPut, "/lists/"+url.PathEscape(id), params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, "Updated list "+id), nil
}
