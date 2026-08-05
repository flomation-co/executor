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
	{Name: "block_ids", Type: core.ConnectionTypeObject, Label: "Block IDs (JSON array)"},
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

	summary, blockIDs := summariseBlocks(blockID, result.Results)

	return map[string]interface{}{
		"tool_result": summary,
		"blocks":      result.Results,
		"block_ids":   blockIDs,
		"count":       int64(len(result.Results)),
		"success":     true,
		"error":       "",
	}, nil
}

// summariseBlocks renders the AI-/human-readable block listing (one line per
// block, each carrying its ID so a caller can target it for update/delete/
// reorder) and returns the collected block IDs in order.
func summariseBlocks(blockID string, results []interface{}) (string, []string) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Block %s has %d child block(s):\n\n", blockID, len(results)))
	ids := make([]string, 0, len(results))
	for _, r := range results {
		bm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		blockType, _ := bm["type"].(string)
		id, _ := bm["id"].(string)
		if id != "" {
			ids = append(ids, id)
		}
		// A "(has children)" hint flags blocks whose own children need a
		// further get_block_children call to reach.
		childHint := ""
		if hc, _ := bm["has_children"].(bool); hc {
			childHint = " (has children)"
		}
		// Include the block ID on every line so a caller (or the AI) can target
		// this exact block.
		if text := extractBlockText(bm, blockType); text != "" {
			sb.WriteString(fmt.Sprintf("[%s] %s  (id: %s)%s\n", blockType, text, id, childHint))
		} else {
			sb.WriteString(fmt.Sprintf("[%s]  (id: %s)%s\n", blockType, id, childHint))
		}
	}
	return sb.String(), ids
}

func extractBlockText(block map[string]interface{}, blockType string) string {
	if typeData, ok := block[blockType].(map[string]interface{}); ok {
		if richText, ok := typeData["rich_text"].([]interface{}); ok {
			return notion.ExtractRichText(richText)
		}
	}
	return ""
}
