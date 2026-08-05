// Package notion_update_block edits a single Notion block in place, targeted by
// its block ID (which get_block_children now surfaces). For a text block, pass
// `content` and it rewrites the block's rich text without appending a duplicate;
// for anything else, pass a raw `block_json` body (the Notion PATCH /blocks/{id}
// shape). Together with delete_block this makes the block toolset addressable.
package notion_update_block

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	notion "flomation.app/automate/executor/actions/notion"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Notion Update Block"
	Description  = "Edit a Notion block in place by its ID (rewrite its text or set raw fields)"
	Website      = "https://www.flomation.co"
	Icon         = "notion+pen"
	Date         = "05/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "block_id", Type: core.ConnectionTypeString, Label: "Block ID", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "New text content", Placeholder: "Rewrites a text block's content"},
	{Name: "block_json", Type: core.ConnectionTypeText, Label: "Block body (JSON)", Placeholder: "Advanced: raw Notion PATCH body"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "block", Type: core.ConnectionTypeObject, Label: "Updated block (JSON)"},
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

	var body map[string]interface{}

	// Advanced: a raw PATCH body wins, so any block field can be set.
	if raw := notion.OptionalString("block_json", inputs); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return notion.ErrorResult(fmt.Sprintf("Invalid block JSON: %s", err)), nil
		}
	} else if content := notion.OptionalString("content", inputs); content != "" {
		// Rewrite a text block's rich text. First read the block to learn its
		// type (paragraph, heading_2, quote, …) — Notion's PATCH keys the update
		// by the block's own type, so the wrong key is rejected.
		blockType, terr := fetchBlockType(apiKey, blockID)
		if terr != nil {
			return notion.ErrorResult(terr.Error()), nil
		}
		body = map[string]interface{}{
			blockType: map[string]interface{}{
				"rich_text": []map[string]interface{}{
					{"type": "text", "text": map[string]string{"content": content}},
				},
			},
		}
	} else {
		return notion.ErrorResult("Either content or block_json is required"), nil
	}

	resp, err := notion.ExecuteAPI(apiKey, "PATCH", fmt.Sprintf("/blocks/%s", blockID), body)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to update block: %s", err)), nil
	}
	if err := notion.CheckResponse(resp); err != nil {
		return notion.ErrorResult(err.Error()), nil
	}

	var updated map[string]interface{}
	_ = json.Unmarshal(resp.Body, &updated)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated block %s", blockID),
		"block":       updated,
		"success":     true,
		"error":       "",
	}, nil
}

// fetchBlockType reads a block and returns its Notion type (e.g. "paragraph").
func fetchBlockType(apiKey, blockID string) (string, error) {
	resp, err := notion.ExecuteAPI(apiKey, "GET", fmt.Sprintf("/blocks/%s", blockID), nil)
	if err != nil {
		return "", fmt.Errorf("failed to read block: %s", err)
	}
	if err := notion.CheckResponse(resp); err != nil {
		return "", err
	}
	var b struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(resp.Body, &b); err != nil {
		return "", fmt.Errorf("failed to parse block: %s", err)
	}
	if b.Type == "" {
		return "", fmt.Errorf("block %s has no type", blockID)
	}
	return b.Type, nil
}
