package notion_list_comments

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
	Name         = "Notion List Comments"
	Description  = "List comments on a Notion page or block"
	Website      = "https://www.flomation.co"
	Icon         = "notion+comments"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "block_id", Type: core.ConnectionTypeString, Label: "Block or Page ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "comments", Type: core.ConnectionTypeObject, Label: "Comments (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := notion.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	blockID, err := notion.RequiredString("block_id", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := notion.ExecuteAPI(apiKey, "GET", fmt.Sprintf("/comments?block_id=%s", blockID), nil)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to list comments: %s", err)), nil
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
	sb.WriteString(fmt.Sprintf("%d comment(s) on %s:\n", len(result.Results), blockID))
	for _, r := range result.Results {
		if cm, ok := r.(map[string]interface{}); ok {
			if rt, ok := cm["rich_text"].([]interface{}); ok {
				text := notion.ExtractRichText(rt)
				if len(text) > 200 {
					text = text[:200] + "..."
				}
				createdBy := ""
				if cb, ok := cm["created_by"].(map[string]interface{}); ok {
					createdBy, _ = cb["id"].(string)
				}
				sb.WriteString(fmt.Sprintf("- [%s]: %s\n", createdBy, text))
			}
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"comments":    result.Results,
		"count":       int64(len(result.Results)),
		"success":     true,
		"error":       "",
	}, nil
}
