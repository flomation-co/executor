package trello_checklist_delete

import (
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Delete Checklist"
	Description  = "Delete a checklist from a Trello card."
	Website      = "https://www.flomation.co"
	Icon         = "trello+trash"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "card_id", Type: core.ConnectionTypeString, Label: "Card ID", Placeholder: "The card the checklist belongs to", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Checklist ID", Placeholder: "The ID of the checklist to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Checklist ID"},
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
	cardID, err := trello.RequiredString("card_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	id, err := trello.RequiredString("id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	// This delete returns a JSON array (the card's remaining checklists), so use
	// the status-only helper rather than decoding an object.
	if err := trello.DeleteOK(auth, "/cards/"+url.PathEscape(cardID)+"/checklists/"+url.PathEscape(id), url.Values{}); err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.SuccessResult(id, nil, "Deleted checklist "+id), nil
}
