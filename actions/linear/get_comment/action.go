package linear_get_comment

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Comment"
	Description  = "Retrieve a single Linear comment by ID with full body text."
	Website      = "https://www.flomation.co"
	Icon         = "linear"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Linear API Key", Placeholder: "lin_api_...", Required: true},
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID", Placeholder: "Comment UUID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "body", Type: core.ConnectionTypeString, Label: "Comment Body"},
	{Name: "author", Type: core.ConnectionTypeString, Label: "Author"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Created At"},
	{Name: "issue_id", Type: core.ConnectionTypeString, Label: "Issue ID"},
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
		Query: `query Comment($id: String!) {
			comment(id: $id) {
				id
				body
				createdAt
				updatedAt
				user {
					name
					displayName
				}
				issue {
					id
					identifier
				}
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
		Comment struct {
			ID        string `json:"id"`
			Body      string `json:"body"`
			CreatedAt string `json:"createdAt"`
			User      struct {
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
			} `json:"user"`
			Issue struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
			} `json:"issue"`
		} `json:"comment"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	c := result.Comment
	author := c.User.DisplayName
	if author == "" {
		author = c.User.Name
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Comment by %s on %s (%s):\n\n%s", author, c.Issue.Identifier, c.CreatedAt, c.Body),
		"body":        c.Body,
		"author":      author,
		"created_at":  c.CreatedAt,
		"issue_id":    c.Issue.ID,
		"success":     true,
		"error":       "",
	}, nil
}
