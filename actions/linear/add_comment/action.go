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
	Name         = "Add Comment"
	Description  = "Add a Markdown comment to an existing Linear issue."
	Website      = "https://www.flomation.co"
	Icon         = "linear"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	linear.AuthInputs[0],
	{
		Name:        "issue_id",
		Type:        core.ConnectionTypeString,
		Label:       "Issue ID",
		Placeholder: "Issue UUID",
		Required:    true,
	},
	{
		Name:        "body",
		Type:        core.ConnectionTypeText,
		Label:       "Comment Body",
		Placeholder: "Markdown comment text",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID"},
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
				comment {
					id
				}
			}
		}`,
		Variables: map[string]interface{}{
			"input": map[string]interface{}{
				"issueId": issueID,
				"body":    body,
			},
		},
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		CommentCreate struct {
			Success bool `json:"success"`
			Comment struct {
				ID string `json:"id"`
			} `json:"comment"`
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
