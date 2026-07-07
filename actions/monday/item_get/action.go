package monday_item_get

import (
	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Item"
	Description  = "Fetch a single Monday.com item by its ID, including its column values."
	Website      = "https://www.flomation.co"
	Icon         = "monday+magnifying-glass"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Placeholder: "The ID of the item to fetch", Required: true},
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
	data, err := monday.GraphQL(auth, `query ($itemId: [ID!]) { items (ids: $itemId) `+monday.ItemFields+` }`,
		map[string]interface{}{"itemId": []string{itemID}})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	items := monday.ArrayField(data, "items")
	if len(items) == 0 {
		return monday.ErrorResult("item " + itemID + " not found"), nil
	}
	obj, _ := items[0].(map[string]interface{})
	return monday.ResourceResult(obj, "Retrieved item "+itemID), nil
}
