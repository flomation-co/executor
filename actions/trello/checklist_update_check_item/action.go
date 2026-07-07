package trello_checklist_update_check_item

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Checklist Item"
	Description  = "Update a checklist item on a Trello card — rename it, mark it complete/incomplete, or reposition it."
	Website      = "https://www.flomation.co"
	Icon         = "trello+pen"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "card_id", Type: core.ConnectionTypeString, Label: "Card ID", Placeholder: "The card the checklist item is on", Required: true},
	{Name: "check_item_id", Type: core.ConnectionTypeString, Label: "Checklist Item ID", Placeholder: "The ID of the checklist item to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A new text for the item"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State", Placeholder: "Mark the item complete or incomplete", Options: []core.ConnectionOption{
		{Name: "Complete", Value: "complete"},
		{Name: "Incomplete", Value: "incomplete"},
	}},
	{Name: "position", Type: core.ConnectionTypeString, Label: "Position", Placeholder: "top, bottom, or a positive number"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Checklist Item ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Checklist Item"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := trello.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	cardID, err := trello.RequiredString("card_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	checkItemID, err := trello.RequiredString("check_item_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	trello.SetIfPresent(params, inputs, "name", "name")
	trello.SetIfPresent(params, inputs, "state", "state")
	trello.SetIfPresent(params, inputs, "pos", "position")
	obj, err := trello.WriteObject(auth, http.MethodPut, "/cards/"+url.PathEscape(cardID)+"/checkItem/"+url.PathEscape(checkItemID), params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, "Updated checklist item "+checkItemID), nil
}
