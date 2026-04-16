package linear_list_issues

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Issues"
	Description  = "List and filter Linear issues by team, state, assignee, priority, or label."
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
		Name:        "team_id",
		Type:        core.ConnectionTypeString,
		Label:       "Team ID",
		Placeholder: "Filter by team UUID (optional)",
	},
	{
		Name:        "assignee_id",
		Type:        core.ConnectionTypeString,
		Label:       "Assignee ID",
		Placeholder: "Filter by assignee UUID (optional)",
	},
	{
		Name:        "state_name",
		Type:        core.ConnectionTypeString,
		Label:       "State Name",
		Placeholder: "Filter by state name, e.g. In Progress (optional)",
	},
	{
		Name:  "priority",
		Type:  core.ConnectionTypeString,
		Label: "Priority",
		Options: []core.ConnectionOption{
			{Name: "Any", Value: ""},
			{Name: "Urgent", Value: "1"},
			{Name: "High", Value: "2"},
			{Name: "Medium", Value: "3"},
			{Name: "Low", Value: "4"},
		},
	},
	{
		Name:        "label_name",
		Type:        core.ConnectionTypeString,
		Label:       "Label Name",
		Placeholder: "Filter by label name (optional)",
	},
	{
		Name:        "project_id",
		Type:        core.ConnectionTypeString,
		Label:       "Project ID",
		Placeholder: "Filter by project UUID (optional)",
	},
	{
		Name:        "limit",
		Type:        core.ConnectionTypeInteger,
		Label:       "Limit",
		Placeholder: "50",
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

	limit := int64(50)
	if conn := core.FindConnection("limit", inputs); conn != nil && conn.Number() != nil && *conn.Number() > 0 {
		limit = *conn.Number()
		if limit > 250 {
			limit = 250
		}
	}

	// Build filter object
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
	if v := linear.OptionalString("project_id", inputs); v != "" {
		filter["project"] = map[string]interface{}{"id": map[string]string{"eq": v}}
	}

	vars := map[string]interface{}{
		"first": limit,
	}
	if len(filter) > 0 {
		vars["filter"] = filter
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query ListIssues($first: Int, $filter: IssueFilter) {
			issues(first: $first, filter: $filter, orderBy: updatedAt) {
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
		Variables: vars,
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		Issues struct {
			Nodes []interface{} `json:"nodes"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d issues", len(result.Issues.Nodes)),
		"issues":      result.Issues.Nodes,
		"count":       len(result.Issues.Nodes),
		"success":     true,
		"error":       "",
	}, nil
}
