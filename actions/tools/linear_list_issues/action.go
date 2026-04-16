// Package linear_list_issues is a tool wrapper for the linear/list_issues
// action, making it available as an AI agent tool.
package linear_list_issues

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
	Name         = "Linear List Issues"
	Description  = "List and filter issues in Linear. Supports filtering by team, assignee, state, priority, and label. Returns a summary of matching issues."
	Website      = "https://www.flomation.co"
	Icon    = "linear"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Linear API key", Required: true},
	{Name: "team_id", Type: core.ConnectionTypeString, Label: "Filter by team UUID"},
	{Name: "assignee_id", Type: core.ConnectionTypeString, Label: "Filter by assignee UUID"},
	{Name: "state_name", Type: core.ConnectionTypeString, Label: "Filter by state name (e.g. In Progress, Done)"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "Filter by priority: 1=urgent, 2=high, 3=medium, 4=low"},
	{Name: "label_name", Type: core.ConnectionTypeString, Label: "Filter by label name"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum number of issues to return (default 20)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Issue list summary for the AI"},
	{Name: "issues", Type: core.ConnectionTypeObject, Label: "Full issue data array"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Number of issues returned"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	limit := int64(20)
	if conn := core.FindConnection("limit", inputs); conn != nil && conn.Number() != nil && *conn.Number() > 0 {
		limit = *conn.Number()
		if limit > 100 {
			limit = 100
		}
	}

	filter := map[string]interface{}{}
	if v := linear.OptionalString("team_id", inputs); v != "" {
		filter["team"] = map[string]interface{}{"id": map[string]string{"eq": v}}
	}
	if v := linear.OptionalString("assignee_id", inputs); v != "" {
		filter["assignee"] = map[string]interface{}{"id": map[string]string{"eq": v}}
	}
	if v := linear.OptionalString("state_name", inputs); v != "" {
		filter["state"] = map[string]interface{}{"name": map[string]string{"eq": v}}
	}
	if v := linear.OptionalString("priority", inputs); v != "" {
		var p int
		fmt.Sscanf(v, "%d", &p)
		filter["priority"] = map[string]interface{}{"eq": p}
	}
	if v := linear.OptionalString("label_name", inputs); v != "" {
		filter["labels"] = map[string]interface{}{"name": map[string]string{"eq": v}}
	}

	vars := map[string]interface{}{"first": limit}
	if len(filter) > 0 {
		vars["filter"] = filter
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query ListIssues($first: Int, $filter: IssueFilter) {
			issues(first: $first, filter: $filter, orderBy: updatedAt) {
				nodes {
					identifier title priorityLabel
					state { name }
					assignee { name }
					team { key }
				}
			}
		}`,
		Variables: vars,
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to list issues: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		Issues struct {
			Nodes []struct {
				Identifier    string `json:"identifier"`
				Title         string `json:"title"`
				PriorityLabel string `json:"priorityLabel"`
				State         *struct{ Name string } `json:"state"`
				Assignee      *struct{ Name string } `json:"assignee"`
				Team          *struct{ Key string } `json:"team"`
			} `json:"nodes"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var lines []string
	for _, issue := range result.Issues.Nodes {
		state := "?"
		if issue.State != nil {
			state = issue.State.Name
		}
		assignee := "unassigned"
		if issue.Assignee != nil {
			assignee = issue.Assignee.Name
		}
		lines = append(lines, fmt.Sprintf("• %s: %s [%s] %s — %s",
			issue.Identifier, issue.Title, state, issue.PriorityLabel, assignee))
	}

	summary := fmt.Sprintf("Found %d issues:", len(result.Issues.Nodes))
	if len(lines) > 0 {
		summary += "\n" + strings.Join(lines, "\n")
	}

	// Full data for downstream nodes
	fullJSON, _ := json.Marshal(result.Issues.Nodes)
	var fullObj interface{}
	_ = json.Unmarshal(fullJSON, &fullObj)

	return map[string]interface{}{
		"tool_result": summary,
		"issues":      fullObj,
		"count":       len(result.Issues.Nodes),
		"success":     true,
		"error":       "",
	}, nil
}
