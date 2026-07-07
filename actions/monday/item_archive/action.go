package monday_item_archive

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Archive Item"
	Description  = "Archive a Monday.com item by its ID (a softer alternative to deleting it)."
	Website      = "https://www.flomation.co"
	Icon         = "monday+box-archive"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Placeholder: "The ID of the item to archive", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Item ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Item"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := monday.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	itemID, err := monday.RequiredString("item_id", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	data, err := monday.GraphQL(auth, `mutation ($itemId: ID!) { archive_item (item_id: $itemId) { id } }`,
		map[string]interface{}{"itemId": itemID})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ResourceResult(monday.ObjectField(data, "archive_item"), "Archived item "+itemID), nil
}
