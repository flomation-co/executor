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
	Name         = "Search Issues"
	Description  = "Full-text search across Linear issues by keyword."
	Website      = "https://www.flomation.co"
	Icon         = "linear+magnifying-glass"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "Linear API Key",
		Placeholder: "lin_api_...",
		Required:    true,
	},
	{
		Name:        "query",
		Type:        core.ConnectionTypeSecret,
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
		Query: `query SearchIssues($term: String!, $first: Int) {
			searchIssues(term: $term, first: $first) {
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
					project { id name }
					labels { nodes { name } }
				}
			}
		}`,
		Variables: map[string]interface{}{
			"term":  query,
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
		SearchIssues struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"searchIssues"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	type issueRow struct {
		ID            string `json:"id"`
		Identifier    string `json:"identifier"`
		Title         string `json:"title"`
		PriorityLabel string `json:"priorityLabel"`
		DueDate       string `json:"dueDate"`
		State         struct {
			Name string `json:"name"`
		} `json:"state"`
		Assignee *struct {
			Name string `json:"name"`
		} `json:"assignee"`
		Project *struct {
			Name string `json:"name"`
		} `json:"project"`
		Labels struct {
			Nodes []struct {
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"labels"`
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d issue(s):\n\n", len(result.SearchIssues.Nodes))
	var parsed []interface{}
	for _, raw := range result.SearchIssues.Nodes {
		var row issueRow
		if err := json.Unmarshal(raw, &row); err == nil {
			assignee := "Unassigned"
			if row.Assignee != nil && row.Assignee.Name != "" {
				assignee = row.Assignee.Name
			}
			due := ""
			if row.DueDate != "" {
				due = fmt.Sprintf(" due:%s", row.DueDate)
			}
			var labels []string
			for _, l := range row.Labels.Nodes {
				labels = append(labels, l.Name)
			}
			labelStr := ""
			if len(labels) > 0 {
				labelStr = fmt.Sprintf(" [%s]", strings.Join(labels, ", "))
			}
			project := "No project"
			if row.Project != nil && row.Project.Name != "" {
				project = row.Project.Name
			}
			fmt.Fprintf(&sb, "• [%s] %s (id:%s, %s, %s, %s, project:%s%s)%s\n",
				row.Identifier, row.Title, row.ID, row.State.Name, row.PriorityLabel, assignee, project, due, labelStr)
		}
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		parsed = append(parsed, generic)
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"issues":      parsed,
		"count":       len(result.SearchIssues.Nodes),
		"success":     true,
		"error":       "",
	}, nil
}
