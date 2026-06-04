package notion_query_database

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
	Name         = "Notion Query Database"
	Description  = "Query a Notion database with optional filters and sorts"
	Website      = "https://www.flomation.co"
	Icon         = "notion+magnifying-glass"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "database_id", Type: core.ConnectionTypeString, Label: "Database ID", Required: true},
	{Name: "filter_json", Type: core.ConnectionTypeText, Label: "Filter (JSON)", Placeholder: "{\"property\": \"Status\", \"select\": {\"equals\": \"Done\"}}"},
	{Name: "sorts_json", Type: core.ConnectionTypeText, Label: "Sorts (JSON array)", Placeholder: "[{\"property\": \"Created\", \"direction\": \"descending\"}]"},
	{Name: "page_size", Type: core.ConnectionTypeString, Label: "Results (default 20, max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Results (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	body := map[string]interface{}{}

	if filterJSON := notion.OptionalString("filter_json", inputs); filterJSON != "" {
		var filter interface{}
		if err := json.Unmarshal([]byte(filterJSON), &filter); err == nil {
			body["filter"] = filter
		}
	}
	if sortsJSON := notion.OptionalString("sorts_json", inputs); sortsJSON != "" {
		var sorts interface{}
		if err := json.Unmarshal([]byte(sortsJSON), &sorts); err == nil {
			body["sorts"] = sorts
		}
	}
	if ps := notion.OptionalString("page_size", inputs); ps != "" {
		var n int
		fmt.Sscanf(ps, "%d", &n)
		if n > 0 {
			body["page_size"] = n
		}
	}

	resp, err := notion.ExecuteAPI(apiKey, "POST", fmt.Sprintf("/databases/%s/query", dbID), body)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to query database: %s", err)), nil
	}
	if err := notion.CheckResponse(resp); err != nil {
		return notion.ErrorResult(err.Error()), nil
	}

	var result struct {
		Results []interface{} `json:"results"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Database %s — %d row(s):\n", dbID, len(result.Results)))
	for _, r := range result.Results {
		if rm, ok := r.(map[string]interface{}); ok {
			id, _ := rm["id"].(string)
			url, _ := rm["url"].(string)
			title := ""
			if props, ok := rm["properties"].(map[string]interface{}); ok {
				title = notion.ExtractTitle(props)
			}
			sb.WriteString(fmt.Sprintf("- %s (ID: %s) %s\n", title, id, url))
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"results":     result.Results,
		"count":       int64(len(result.Results)),
		"success":     true,
		"error":       "",
	}, nil
}
