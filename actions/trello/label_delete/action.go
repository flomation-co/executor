package trello_label_delete

import (
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Delete Label"
	Description  = "Delete a Trello label by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "trello+trash"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board that owns the label (used to load the label picker)"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Label", Placeholder: "The label to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Label ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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
	if err := trello.DeleteOK(auth, "/labels/"+url.PathEscape(id), url.Values{}); err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.SuccessResult(id, nil, "Deleted label "+id), nil
}
