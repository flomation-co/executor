package monday_item_add_update

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Add Update to Item"
	Description  = "Post an update (a comment) on a Monday.com item."
	Website      = "https://www.flomation.co"
	Icon         = "monday+comment"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Placeholder: "The item to post an update on", Required: true},
	{Name: "value", Type: core.ConnectionTypeText, Label: "Update", Placeholder: "The update text", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Update ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Update"},
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
	value, err := monday.RequiredString("value", inputs)
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	data, err := monday.GraphQL(auth, `mutation ($itemId: ID!, $value: String!) { create_update (item_id: $itemId, body: $value) { id } }`,
		map[string]interface{}{"itemId": itemID, "value": value})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	return monday.ResourceResult(monday.ObjectField(data, "create_update"), "Added update to item "+itemID), nil
}
