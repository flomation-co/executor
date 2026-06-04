package notion_get_database

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	notion "flomation.app/automate/executor/actions/notion"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Notion Get Database"
	Description  = "Retrieve a Notion database schema and metadata"
	Website      = "https://www.flomation.co"
	Icon         = "notion+database"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "database_id", Type: core.ConnectionTypeString, Label: "Database ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "URL"},
	{Name: "properties", Type: core.ConnectionTypeObject, Label: "Schema (JSON)"},
	{Name: "data", Type: core.ConnectionTypeObject, Label: "Full Response (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := notion.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	dbID, err := notion.RequiredString("database_id", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := notion.ExecuteAPI(apiKey, "GET", fmt.Sprintf("/databases/%s", dbID), nil)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to get database: %s", err)), nil
	}
	if err := notion.CheckResponse(resp); err != nil {
		return notion.ErrorResult(err.Error()), nil
	}

	var db map[string]interface{}
	if err := json.Unmarshal(resp.Body, &db); err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	title := ""
	if t, ok := db["title"].([]interface{}); ok {
		title = notion.ExtractRichText(t)
	}
	dbURL, _ := db["url"].(string)

	// Build schema summary
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Database: %s\nID: %s\nURL: %s\n\nProperties:\n", title, dbID, dbURL))
	if props, ok := db["properties"].(map[string]interface{}); ok {
		for name, v := range props {
			if pm, ok := v.(map[string]interface{}); ok {
				propType, _ := pm["type"].(string)
				sb.WriteString(fmt.Sprintf("- %s (%s)\n", name, propType))
			}
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"title":       title,
		"url":         dbURL,
		"properties":  db["properties"],
		"data":        db,
		"success":     true,
		"error":       "",
	}, nil
}
