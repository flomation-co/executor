package notion_delete_block

import (
	"fmt"

	core "flomation.app/automate/executor"
	notion "flomation.app/automate/executor/actions/notion"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Notion Delete Block"
	Description  = "Delete (archive) a block from a Notion page"
	Website      = "https://www.flomation.co"
	Icon         = "notion"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "block_id", Type: core.ConnectionTypeString, Label: "Block ID", Required: true},
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

	resp, err := notion.ExecuteAPI(apiKey, "DELETE", fmt.Sprintf("/blocks/%s", blockID), nil)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to delete block: %s", err)), nil
	}
	if err := notion.CheckResponse(resp); err != nil {
		return notion.ErrorResult(err.Error()), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted block %s", blockID),
		"success":     true,
		"error":       "",
	}, nil
}
