package webflow_create_collection_item

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Collection Item"
	Description  = "Create a new item in a Webflow CMS collection"
	Website      = "https://www.flomation.co"
	Icon         = "webflow+plus"
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
		Name:        "field_data",
		Type:        core.ConnectionTypeText,
		Label:       "Field Data",
		Placeholder: `{"name": "My Item", "slug": "my-item", ...}`,
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
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID"},
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

	fieldDataStr, err := webflow.RequiredString("field_data", inputs)
	if err != nil {
		return nil, err
	}

	var fieldData interface{}
	if err := json.Unmarshal([]byte(fieldDataStr), &fieldData); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Invalid field_data JSON: %s", err))
	}

	isDraft := webflow.OptionalBool("is_draft", inputs, true)

	reqBody := map[string]interface{}{
		"isArchived": false,
		"isDraft":    isDraft,
		"fieldData":  fieldData,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to build request body: %s", err))
	}

	path := "/collections/" + collectionID + "/items"
	status, body, err := webflow.ExecuteRequest(token, "POST", path, bodyBytes)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to create collection item: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var item struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &item)

	var parsed interface{}
	_ = json.Unmarshal(body, &parsed)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created collection item %s", item.ID),
		"item_id":     item.ID,
		"item":        parsed,
		"success":     true,
		"error":       "",
	}, nil
}
