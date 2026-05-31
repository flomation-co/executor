package webflow_update_collection_item

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Update Collection Item"
	Description  = "Update an existing item in a Webflow CMS collection"
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
	{
		Name:        "field_data",
		Type:        core.ConnectionTypeText,
		Label:       "Field Data",
		Placeholder: `{"name": "Updated Item", ...}`,
		Required:    true,
	},
	{
		Name:  "is_draft",
		Type:  core.ConnectionTypeBoolean,
		Label: "Is Draft",
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

	fieldDataStr, err := webflow.RequiredString("field_data", inputs)
	if err != nil {
		return nil, err
	}

	var fieldData interface{}
	if err := json.Unmarshal([]byte(fieldDataStr), &fieldData); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Invalid field_data JSON: %s", err))
	}

	reqBody := map[string]interface{}{
		"isArchived": false,
		"fieldData":  fieldData,
	}

	// Only include isDraft if explicitly provided
	conn := core.FindConnection("is_draft", inputs)
	if conn != nil && conn.Boolean() != nil {
		reqBody["isDraft"] = *conn.Boolean()
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to build request body: %s", err))
	}

	path := "/collections/" + collectionID + "/items/" + itemID
	status, body, err := webflow.ExecuteRequest(token, "PATCH", path, bodyBytes)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to update collection item: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var parsed interface{}
	_ = json.Unmarshal(body, &parsed)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated collection item %s", itemID),
		"item":        parsed,
		"success":     true,
		"error":       "",
	}, nil
}
