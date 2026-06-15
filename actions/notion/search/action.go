package notion_search

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
	Name         = "Notion Search"
	Description  = "Search across all pages and databases in a Notion workspace"
	Website      = "https://www.flomation.co"
	Icon         = "notion+magnifying-glass"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search Query", Required: true},
	{Name: "filter_type", Type: core.ConnectionTypeString, Label: "Filter Type", Options: []core.ConnectionOption{
		{Name: "All", Value: ""},
		{Name: "Pages Only", Value: "page"},
		{Name: "Databases Only", Value: "database"},
	}},
	{Name: "page_size", Type: core.ConnectionTypeString, Label: "Results (default 10, max 100)"},
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
	query, err := notion.RequiredString("query", inputs)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"query": query,
	}
	if ft := notion.OptionalString("filter_type", inputs); ft != "" {
		body["filter"] = map[string]string{"value": ft, "property": "object"}
	}
	if ps := notion.OptionalString("page_size", inputs); ps != "" {
		var n int
		fmt.Sscanf(ps, "%d", &n)
		if n > 0 {
			body["page_size"] = n
		}
	}

	resp, err := notion.ExecuteAPI(apiKey, "POST", "/search", body)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to search: %s", err)), nil
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
	sb.WriteString(fmt.Sprintf("Found %d result(s) for %q:\n", len(result.Results), query))
	for _, r := range result.Results {
		if rm, ok := r.(map[string]interface{}); ok {
			objType, _ := rm["object"].(string)
			id, _ := rm["id"].(string)
			url, _ := rm["url"].(string)
			title := ""
			if props, ok := rm["properties"].(map[string]interface{}); ok {
				title = notion.ExtractTitle(props)
			}
			if t, ok := rm["title"].([]interface{}); ok && title == "" {
				title = notion.ExtractRichText(t)
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s (ID: %s) %s\n", objType, title, id, url))
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
