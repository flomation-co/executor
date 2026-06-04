package linear_list_comments

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Comments"
	Description  = "List comments on a Linear issue. Returns comment IDs, authors, and body text."
	Website      = "https://www.flomation.co"
	Icon         = "linear+comments"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Linear API Key", Placeholder: "lin_api_...", Required: true},
	{Name: "issue_id", Type: core.ConnectionTypeString, Label: "Issue ID", Placeholder: "Issue UUID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "comments", Type: core.ConnectionTypeObject, Label: "Comments (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query IssueComments($id: String!) {
			issue(id: $id) {
				comments {
					nodes {
						id
						body
						createdAt
						updatedAt
						user {
							id
							name
							displayName
						}
					}
				}
			}
		}`,
		Variables: map[string]interface{}{"id": issueID},
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		Issue struct {
			Comments struct {
				Nodes []struct {
					ID        string `json:"id"`
					Body      string `json:"body"`
					CreatedAt string `json:"createdAt"`
					User      struct {
						Name        string `json:"name"`
						DisplayName string `json:"displayName"`
					} `json:"user"`
				} `json:"nodes"`
			} `json:"comments"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	comments := result.Issue.Comments.Nodes

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Issue %s has %d comment(s):\n", issueID, len(comments)))
	for _, c := range comments {
		author := c.User.DisplayName
		if author == "" {
			author = c.User.Name
		}
		body := c.Body
		if len(body) > 150 {
			body = body[:150] + "..."
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s (ID: %s): %s\n", c.CreatedAt, author, c.ID, body))
	}

	// Build JSON-friendly output
	var commentsJSON []interface{}
	for _, c := range comments {
		commentsJSON = append(commentsJSON, map[string]interface{}{
			"id":         c.ID,
			"body":       c.Body,
			"created_at": c.CreatedAt,
			"author":     c.User.DisplayName,
		})
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"comments":    commentsJSON,
		"count":       int64(len(comments)),
		"success":     true,
		"error":       "",
	}, nil
}
