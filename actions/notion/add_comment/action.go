package notion_add_comment

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	notion "flomation.app/automate/executor/actions/notion"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Notion Add Comment"
	Description  = "Add a comment to a Notion page or discussion thread"
	Website      = "https://www.flomation.co"
	Icon         = "notion+comments"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Notion Integration Token", Placeholder: "ntn_...", Required: true},
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Page ID", Placeholder: "Page to comment on"},
	{Name: "discussion_id", Type: core.ConnectionTypeString, Label: "Discussion Thread ID", Placeholder: "Reply to existing discussion (optional)"},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Comment Text", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := notion.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	commentBody, err := notion.RequiredString("body", inputs)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"rich_text": []map[string]interface{}{
			{"type": "text", "text": map[string]string{"content": commentBody}},
		},
	}

	// Either page_id (new top-level comment) or discussion_id (reply to thread)
	if discussionID := notion.OptionalString("discussion_id", inputs); discussionID != "" {
		payload["discussion_id"] = discussionID
	} else if pageID := notion.OptionalString("page_id", inputs); pageID != "" {
		payload["parent"] = map[string]string{"page_id": pageID}
	} else {
		return notion.ErrorResult("Either page_id or discussion_id is required"), nil
	}

	resp, err := notion.ExecuteAPI(apiKey, "POST", "/comments", payload)
	if err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to add comment: %s", err)), nil
	}
	if err := notion.CheckResponse(resp); err != nil {
		return notion.ErrorResult(err.Error()), nil
	}

	var comment struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &comment); err != nil {
		return notion.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Added comment %s", comment.ID),
		"comment_id":  comment.ID,
		"success":     true,
		"error":       "",
	}, nil
}
