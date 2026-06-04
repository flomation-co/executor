package notion_create_page

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	notion "flomation.app/automate/executor/actions/notion"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Notion Create Page"
	Description  = "Create a new page in a Notion database or as a child of another page"
	Website      = "https://www.flomation.co"
	Icon         = "notion+plus"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Parent ID (database or page)", Required: true},
	{Name: "parent_type", Type: core.ConnectionTypeString, Label: "Parent Type", Required: true, Options: []core.ConnectionOption{
		{Name: "Database", Value: "database_id"},
		{Name: "Page", Value: "page_id"},
	}},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Page Title", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Page Content (Markdown-style text)"},
	{Name: "properties_json", Type: core.ConnectionTypeText, Label: "Additional Properties (JSON)", Placeholder: "{\"Status\": {\"select\": {\"name\": \"In Progress\"}}}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Page ID"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := notion.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	parentID, err := notion.RequiredString("parent_id", inputs)
	if err != nil {
		return nil, err
	}
	parentType, err := notion.RequiredString("parent_type", inputs)
	if err != nil {
		return nil, err
	}
	title, err := notion.RequiredString("title", inputs)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"parent": map[string]string{parentType: parentID},
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"title": []map[string]interface{}{
					{"text": map[string]string{"content": title}},
				},
			},
		},
	}

	// Merge additional properties if provided
	if propsJSON := notion.OptionalString("properties_json", inputs); propsJSON != "" {
		var extraProps map[string]interface{}
		if err := json.Unmarshal([]byte(propsJSON), &extraProps); err == nil {
			props := body["properties"].(map[string]interface{})
			for k, v := range extraProps {
				props[k] = v
			}
		}
	}

	// Add content as paragraph blocks
	if content := notion.OptionalString("content", inputs); content != "" {
		body["children"] = []map[string]interface{}{
			{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{"type": "text", "text": map[string]string{"content": content}},
					},
				},
			},
		}
	}

	resp, err := notion.ExecuteAPI(apiKey, "POST", "/pages", body)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to create page: %s", err)), nil
	}
	if err := notion.CheckResponse(resp); err != nil {
		return notion.ErrorResult(err.Error()), nil
	}

	var page struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created page \"%s\" — %s", title, page.URL),
		"page_id":     page.ID,
		"url":         page.URL,
		"success":     true,
		"error":       "",
	}, nil
}
