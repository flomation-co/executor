package linear_delete_comment

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Delete Comment"
	Description  = "Delete a comment from a Linear issue by comment ID."
	Website      = "https://www.flomation.co"
	Icon         = "linear+trash"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Linear API Key", Placeholder: "lin_api_...", Required: true},
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID", Placeholder: "Comment UUID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	commentID, err := linear.RequiredString("comment_id", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `mutation CommentDelete($id: String!) {
			commentDelete(id: $id) {
				success
			}
		}`,
		Variables: map[string]interface{}{"id": commentID},
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		CommentDelete struct {
			Success bool `json:"success"`
		} `json:"commentDelete"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.CommentDelete.Success {
		return map[string]interface{}{
			"tool_result": "Failed to delete comment",
			"success":     false,
			"error":       "delete operation returned success=false",
		}, nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted comment %s", commentID),
		"success":     true,
		"error":       "",
	}, nil
}
