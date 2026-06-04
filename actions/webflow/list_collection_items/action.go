package webflow_list_collection_items

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Collection Items"
	Description  = "List items in a Webflow CMS collection with pagination"
	Website      = "https://www.flomation.co"
	Icon         = "webflow+list"
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
		Name:        "limit",
		Type:        core.ConnectionTypeInteger,
		Label:       "Limit",
		Placeholder: "Number of items to return",
	},
	{
		Name:        "offset",
		Type:        core.ConnectionTypeInteger,
		Label:       "Offset",
		Placeholder: "Number of items to skip",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "items", Type: core.ConnectionTypeObject, Label: "Items"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	path := "/collections/" + collectionID + "/items"
	var params []string
	if limit := webflow.OptionalInt("limit", inputs); limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if offset := webflow.OptionalInt("offset", inputs); offset > 0 {
		params = append(params, fmt.Sprintf("offset=%d", offset))
	}
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}

	status, body, err := webflow.ExecuteRequest(token, "GET", path, nil)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to list collection items: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err))
	}

	var parsed []interface{}
	for _, raw := range resp.Items {
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		parsed = append(parsed, generic)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d item(s) in collection", len(resp.Items)),
		"items":       parsed,
		"count":       len(resp.Items),
		"success":     true,
		"error":       "",
	}, nil
}
