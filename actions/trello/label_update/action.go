package trello_label_update

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Label"
	Description  = "Change a Trello label's name or color."
	Website      = "https://www.flomation.co"
	Icon         = "trello+pen"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "board_id", Type: core.ConnectionTypeString, Label: "Board", Placeholder: "The board that owns the label (used to load the label picker)"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Label", Placeholder: "The label to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A new name for the label"},
	{Name: "color", Type: core.ConnectionTypeString, Label: "Color", Placeholder: "The label color", Options: []core.ConnectionOption{
		{Name: "None", Value: "null"},
		{Name: "Green", Value: "green"},
		{Name: "Yellow", Value: "yellow"},
		{Name: "Orange", Value: "orange"},
		{Name: "Red", Value: "red"},
		{Name: "Purple", Value: "purple"},
		{Name: "Blue", Value: "blue"},
		{Name: "Sky", Value: "sky"},
		{Name: "Lime", Value: "lime"},
		{Name: "Pink", Value: "pink"},
		{Name: "Black", Value: "black"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Label ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Label"},
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
	trello.SetIfPresent(params, inputs, "color", "color")
	obj, err := trello.WriteObject(auth, http.MethodPut, "/labels/"+url.PathEscape(id), params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, "Updated label "+id), nil
}
