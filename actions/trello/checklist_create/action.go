package trello_checklist_create

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Checklist"
	Description  = "Add a checklist to a Trello card."
	Website      = "https://www.flomation.co"
	Icon         = "trello+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "card_id", Type: core.ConnectionTypeString, Label: "Card ID", Placeholder: "The card to add the checklist to", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The name of the checklist", Required: true},
	{Name: "position", Type: core.ConnectionTypeString, Label: "Position", Placeholder: "top, bottom, or a positive number"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "Extra Trello fields as JSON, e.g. {\"idChecklistSource\":\"...\"}"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Checklist ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Checklist"},
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
	name, err := trello.RequiredString("name", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	params.Set("name", name)
	trello.SetIfPresent(params, inputs, "pos", "position")
	if err := trello.MergeAdditionalFields(params, inputs); err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	obj, err := trello.WriteObject(auth, http.MethodPost, "/cards/"+url.PathEscape(cardID)+"/checklists", params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, "Created checklist "+name), nil
}
