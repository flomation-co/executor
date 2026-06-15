package notion_update_page

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	notion "flomation.app/automate/executor/actions/notion"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Notion Update Page"
	Description  = "Update properties of an existing Notion page"
	Website      = "https://www.flomation.co"
	Icon         = "notion+pencil"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Page ID", Required: true},
	{Name: "properties_json", Type: core.ConnectionTypeText, Label: "Properties (JSON)", Placeholder: "{\"Status\": {\"select\": {\"name\": \"Done\"}}}", Required: true},
	{Name: "archived", Type: core.ConnectionTypeBoolean, Label: "Archive Page"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := notion.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	pageID, err := notion.RequiredString("page_id", inputs)
	if err != nil {
		return nil, err
	}
	propsJSON, err := notion.RequiredString("properties_json", inputs)
	if err != nil {
		return nil, err
	}

	var props map[string]interface{}
	if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
		return notion.ErrorResult(fmt.Sprintf("Invalid properties JSON: %s", err)), nil
	}

	body := map[string]interface{}{
		"properties": props,
	}

	conn := core.FindConnection("archived", inputs)
	if conn != nil && conn.Boolean() != nil && *conn.Boolean() {
		body["archived"] = true
	}

	resp, err := notion.ExecuteAPI(apiKey, "PATCH", fmt.Sprintf("/pages/%s", pageID), body)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to update page: %s", err)), nil
	}
	if err := notion.CheckResponse(resp); err != nil {
		return notion.ErrorResult(err.Error()), nil
	}

	var page struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated page %s — %s", pageID, page.URL),
		"url":         page.URL,
		"success":     true,
		"error":       "",
	}, nil
}
