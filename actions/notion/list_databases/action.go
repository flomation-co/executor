package notion_list_databases

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
	Name         = "Notion List Databases"
	Description  = "List all databases shared with the integration"
	Website      = "https://www.flomation.co"
	Icon         = "notion"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "databases", Type: core.ConnectionTypeObject, Label: "Databases (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := notion.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	// Use search with database filter to list all databases
	body := map[string]interface{}{
		"filter":    map[string]string{"value": "database", "property": "object"},
		"page_size": 100,
	}

	resp, err := notion.ExecuteAPI(apiKey, "POST", "/search", body)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to list databases: %s", err)), nil
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
	sb.WriteString(fmt.Sprintf("Found %d database(s):\n", len(result.Results)))
	for _, r := range result.Results {
		if rm, ok := r.(map[string]interface{}); ok {
			id, _ := rm["id"].(string)
			url, _ := rm["url"].(string)
			title := ""
			if t, ok := rm["title"].([]interface{}); ok {
				title = notion.ExtractRichText(t)
			}
			sb.WriteString(fmt.Sprintf("- %s (ID: %s) %s\n", title, id, url))
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"databases":   result.Results,
		"count":       int64(len(result.Results)),
		"success":     true,
		"error":       "",
	}, nil
}
