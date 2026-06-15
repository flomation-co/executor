package notion_get_page

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	notion "flomation.app/automate/executor/actions/notion"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Notion Get Page"
	Description  = "Retrieve a Notion page's properties by ID"
	Website      = "https://www.flomation.co"
	Icon         = "notion+eye"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Page ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "URL"},
	{Name: "properties", Type: core.ConnectionTypeObject, Label: "Properties (JSON)"},
	{Name: "data", Type: core.ConnectionTypeObject, Label: "Full Response (JSON)"},
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

	resp, err := notion.ExecuteAPI(apiKey, "GET", fmt.Sprintf("/pages/%s", pageID), nil)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to get page: %s", err)), nil
	}
	if err := notion.CheckResponse(resp); err != nil {
		return notion.ErrorResult(err.Error()), nil
	}

	var page map[string]interface{}
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	title := ""
	var properties interface{}
	if props, ok := page["properties"].(map[string]interface{}); ok {
		title = notion.ExtractTitle(props)
		properties = props
	}
	pageURL, _ := page["url"].(string)

	summary := fmt.Sprintf("Page: %s\nID: %s\nURL: %s\n", title, pageID, pageURL)

	return map[string]interface{}{
		"tool_result": summary,
		"title":       title,
		"url":         pageURL,
		"properties":  properties,
		"data":        page,
		"success":     true,
		"error":       "",
	}, nil
}
