package webflow_get_collection_item

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Collection Item"
	Description  = "Get a specific item from a Webflow CMS collection"
	Website      = "https://www.flomation.co"
	Icon         = "globe"
	Date         = "29/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeString,
		Label:       "Webflow API Token",
		Placeholder: "wfl_...",
		Required:    true,
	},
	{
		Name:        "collection_id",
		Type:        core.ConnectionTypeString,
		Label:       "Collection ID",
		Placeholder: "The collection ID",
		Required:    true,
	},
	{
		Name:        "item_id",
		Type:        core.ConnectionTypeString,
		Label:       "Item ID",
		Placeholder: "The item ID",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "item", Type: core.ConnectionTypeObject, Label: "Item"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := webflow.GetAPIToken(inputs)
	if err != nil {
		return nil, err
	}

	collectionID, err := webflow.RequiredString("collection_id", inputs)
	if err != nil {
		return nil, err
	}

	itemID, err := webflow.RequiredString("item_id", inputs)
	if err != nil {
		return nil, err
	}

	path := "/collections/" + collectionID + "/items/" + itemID
	status, body, err := webflow.ExecuteRequest(token, "GET", path, nil)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to get collection item: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var parsed interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err))
	}

	// Extract name from fieldData for the tool result
	var item struct {
		ID        string `json:"id"`
		FieldData struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"fieldData"`
	}
	_ = json.Unmarshal(body, &item)

	toolResult := fmt.Sprintf("Retrieved item %s", item.ID)
	if item.FieldData.Name != "" {
		toolResult = fmt.Sprintf("Retrieved item '%s' (%s)", item.FieldData.Name, item.ID)
	}

	return map[string]interface{}{
		"tool_result": toolResult,
		"item":        parsed,
		"success":     true,
		"error":       "",
	}, nil
}
