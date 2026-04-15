// Package linear_add_comment is a tool wrapper for the linear/add_comment
// action, making it available as an AI agent tool.
package linear_add_comment

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Linear Add Comment"
	Description  = "Add a comment to an existing Linear issue. Provide the issue UUID and a Markdown comment body."
	Website      = "https://www.flomation.co"
	Icon         = "comment"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Linear API key", Required: true},
	{Name: "issue_id", Type: core.ConnectionTypeString, Label: "Issue UUID to comment on", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Comment text (Markdown supported)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary for the AI"},
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment UUID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	issueID, err := linear.RequiredString("issue_id", inputs)
	if err != nil {
		return nil, err
	}
	body, err := linear.RequiredString("body", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `mutation CommentCreate($input: CommentCreateInput!) {
			commentCreate(input: $input) {
				success
				comment { id }
			}
		}`,
		Variables: map[string]interface{}{
			"input": map[string]interface{}{"issueId": issueID, "body": body},
		},
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to add comment: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		CommentCreate struct {
			Success bool `json:"success"`
			Comment struct{ ID string } `json:"comment"`
		} `json:"commentCreate"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return map[string]interface{}{
		"tool_result": "Comment added successfully",
		"comment_id":  result.CommentCreate.Comment.ID,
		"success":     result.CommentCreate.Success,
		"error":       "",
	}, nil
}