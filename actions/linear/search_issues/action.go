package linear_search_issues

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Search Issues"
	Description  = "Full-text search across Linear issues by keyword."
	Website      = "https://www.flomation.co"
	Icon         = "linear"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeString,
		Label:       "Linear API Key",
		Placeholder: "lin_api_...",
		Required:    true,
	},
	{
		Name:        "query",
		Type:        core.ConnectionTypeString,
		Label:       "Search Query",
		Placeholder: "Search keywords",
		Required:    true,
	},
	{
		Name:        "limit",
		Type:        core.ConnectionTypeInteger,
		Label:       "Limit",
		Placeholder: "25",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "issues", Type: core.ConnectionTypeObject, Label: "Issues"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	query, err := linear.RequiredString("query", inputs)
	if err != nil {
		return nil, err
	}

	limit := int64(25)
	if conn := core.FindConnection("limit", inputs); conn != nil && conn.Number() != nil && *conn.Number() > 0 {
		limit = *conn.Number()
		if limit > 250 {
			limit = 250
		}
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query SearchIssues($query: String!, $first: Int) {
			issueSearch(query: $query, first: $first) {
				nodes {
					id
					identifier
					title
					url
					priority
					priorityLabel
					dueDate
					createdAt
					updatedAt
					state { name }
					assignee { name email }
					team { key name }
					labels { nodes { name } }
				}
			}
		}`,
		Variables: map[string]interface{}{
			"query": query,
			"first": limit,
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
		IssueSearch struct {
			Nodes []interface{} `json:"nodes"`
		} `json:"issueSearch"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d issues", len(result.IssueSearch.Nodes)),
		"issues":      result.IssueSearch.Nodes,
		"count":       len(result.IssueSearch.Nodes),
		"success":     true,
		"error":       "",
	}, nil
}
