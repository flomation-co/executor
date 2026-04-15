// Package linear_search_issues is a tool wrapper for the linear/search_issues
// action, making it available as an AI agent tool.
package linear_search_issues

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
	Name         = "Linear Search Issues"
	Description  = "Full-text search across all Linear issues by keyword. Use this to find issues when you know part of the title, description, or identifier."
	Website      = "https://www.flomation.co"
	Icon         = "magnifying-glass"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Linear API key", Required: true},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search keywords", Required: true},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum results (default 10)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Search results for the AI"},
	{Name: "issues", Type: core.ConnectionTypeObject, Label: "Full issue data array"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Number of results"},
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

	limit := int64(10)
	if conn := core.FindConnection("limit", inputs); conn != nil && conn.Number() != nil && *conn.Number() > 0 {
		limit = *conn.Number()
		if limit > 50 {
			limit = 50
		}
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query SearchIssues($query: String!, $first: Int) {
			issueSearch(query: $query, first: $first) {
				nodes {
					identifier title priorityLabel url
					state { name }
					assignee { name }
					team { key }
				}
			}
		}`,
		Variables: map[string]interface{}{"query": query, "first": limit},
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Search failed: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		IssueSearch struct {
			Nodes []struct {
				Identifier    string `json:"identifier"`
				Title         string `json:"title"`
				PriorityLabel string `json:"priorityLabel"`
				URL           string `json:"url"`
				State         *struct{ Name string } `json:"state"`
				Assignee      *struct{ Name string } `json:"assignee"`
			} `json:"nodes"`
		} `json:"issueSearch"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var lines []string
	for _, issue := range result.IssueSearch.Nodes {
		state := "?"
		if issue.State != nil {
			state = issue.State.Name
		}
		lines = append(lines, fmt.Sprintf("• %s: %s [%s] %s", issue.Identifier, issue.Title, state, issue.PriorityLabel))
	}

	summary := fmt.Sprintf("Found %d issues matching \"%s\":", len(result.IssueSearch.Nodes), query)
	if len(lines) > 0 {
		summary += "\n" + strings.Join(lines, "\n")
	}

	fullJSON, _ := json.Marshal(result.IssueSearch.Nodes)
	var fullObj interface{}
	_ = json.Unmarshal(fullJSON, &fullObj)

	return map[string]interface{}{
		"tool_result": summary,
		"issues":      fullObj,
		"count":       len(result.IssueSearch.Nodes),
		"success":     true,
		"error":       "",
	}, nil
}