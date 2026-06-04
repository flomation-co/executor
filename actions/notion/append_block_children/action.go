package notion_append_block_children

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	notion "flomation.app/automate/executor/actions/notion"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Notion Append Content"
	Description  = "Append content blocks to a Notion page or block"
	Website      = "https://www.flomation.co"
	Icon         = "notion+plus"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "block_id", Type: core.ConnectionTypeString, Label: "Block or Page ID", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Text Content", Placeholder: "Text to append as a paragraph"},
	{Name: "blocks_json", Type: core.ConnectionTypeText, Label: "Blocks (JSON array)", Placeholder: "Advanced: raw Notion block objects"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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

	var children []interface{}

	// If raw blocks JSON is provided, use that
	if blocksJSON := notion.OptionalString("blocks_json", inputs); blocksJSON != "" {
		if err := json.Unmarshal([]byte(blocksJSON), &children); err != nil {
			return notion.ErrorResult(fmt.Sprintf("Invalid blocks JSON: %s", err)), nil
		}
	} else if content := notion.OptionalString("content", inputs); content != "" {
		// Otherwise, wrap plain text as a paragraph block
		children = []interface{}{
			map[string]interface{}{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{"type": "text", "text": map[string]string{"content": content}},
					},
				},
			},
		}
	} else {
		return notion.ErrorResult("Either content or blocks_json is required"), nil
	}

	body := map[string]interface{}{
		"children": children,
	}

	resp, err := notion.ExecuteAPI(apiKey, "PATCH", fmt.Sprintf("/blocks/%s/children", blockID), body)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to append content: %s", err)), nil
	}
	if err := notion.CheckResponse(resp); err != nil {
		return notion.ErrorResult(err.Error()), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Appended %d block(s) to %s", len(children), blockID),
		"success":     true,
		"error":       "",
	}, nil
}
