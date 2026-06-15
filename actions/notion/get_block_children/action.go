package notion_get_block_children

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
	Name         = "Notion Get Block Children"
	Description  = "Read the content blocks of a Notion page or block"
	Website      = "https://www.flomation.co"
	Icon         = "notion+list"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "block_id", Type: core.ConnectionTypeString, Label: "Block or Page ID", Required: true},
	{Name: "page_size", Type: core.ConnectionTypeString, Label: "Results (default 50, max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "blocks", Type: core.ConnectionTypeObject, Label: "Blocks (JSON)"},
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

	path := fmt.Sprintf("/blocks/%s/children", blockID)
	if ps := notion.OptionalString("page_size", inputs); ps != "" {
		path += "?page_size=" + ps
	}

	resp, err := notion.ExecuteAPI(apiKey, "GET", path, nil)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to get blocks: %s", err)), nil
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
	sb.WriteString(fmt.Sprintf("Block %s has %d child block(s):\n\n", blockID, len(result.Results)))
	for _, r := range result.Results {
		if bm, ok := r.(map[string]interface{}); ok {
			blockType, _ := bm["type"].(string)
			// Extract text content from common block types
			text := extractBlockText(bm, blockType)
			if text != "" {
				sb.WriteString(fmt.Sprintf("[%s] %s\n", blockType, text))
			} else {
				sb.WriteString(fmt.Sprintf("[%s]\n", blockType))
			}
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"blocks":      result.Results,
		"count":       int64(len(result.Results)),
		"success":     true,
		"error":       "",
	}, nil
}

func extractBlockText(block map[string]interface{}, blockType string) string {
	if typeData, ok := block[blockType].(map[string]interface{}); ok {
		if richText, ok := typeData["rich_text"].([]interface{}); ok {
			return notion.ExtractRichText(richText)
		}
	}
	return ""
}
