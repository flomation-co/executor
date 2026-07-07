package trello_checklist_create_check_item

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	trello "flomation.app/automate/executor/actions/trello"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Checklist Item"
	Description  = "Add an item to a Trello checklist."
	Website      = "https://www.flomation.co"
	Icon         = "trello+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "Your Trello API key", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Trello API token", Required: true},
	{Name: "checklist_id", Type: core.ConnectionTypeString, Label: "Checklist ID", Placeholder: "The checklist to add the item to", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The text of the checklist item", Required: true},
	{Name: "checked", Type: core.ConnectionTypeBoolean, Label: "Checked", Placeholder: "Create the item already checked"},
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
	checklistID, err := trello.RequiredString("checklist_id", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	name, err := trello.RequiredString("name", inputs)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	params := url.Values{}
	params.Set("name", name)
	trello.SetBoolIfSet(params, inputs, "checked", "checked")
	trello.SetIfPresent(params, inputs, "pos", "position")
	obj, err := trello.WriteObject(auth, http.MethodPost, "/checklists/"+url.PathEscape(checklistID)+"/checkItems", params)
	if err != nil {
		return trello.ErrorResult(err.Error()), nil
	}
	return trello.ResourceResult(obj, "Created checklist item "+name), nil
}
