package linear_create_issue

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Issue"
	Description  = "Create a new Linear issue. Requires team_id (use List Teams to find it). Returns identifier and URL."
	Website      = "https://www.flomation.co"
	Icon         = "linear+plus"
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
		Placeholder: "The team UUID or key (e.g. ENG)",
		Required:    true,
	},
	{
		Name:        "title",
		Type:        core.ConnectionTypeString,
		Label:       "Title",
		Placeholder: "Issue title",
		Required:    true,
	},
	{
		Name:        "description",
		Type:        core.ConnectionTypeText,
		Label:       "Description",
		Placeholder: "Markdown description (optional)",
	},
	{
		Name:  "priority",
		Type:  core.ConnectionTypeString,
		Label: "Priority",
		Options: []core.ConnectionOption{
			{Name: "No Priority", Value: "0"},
			{Name: "Urgent", Value: "1"},
			{Name: "High", Value: "2"},
			{Name: "Medium", Value: "3"},
			{Name: "Low", Value: "4"},
		},
	},
	{
		Name:        "assignee_id",
		Type:        core.ConnectionTypeString,
		Label:       "Assignee ID",
		Placeholder: "User UUID (optional)",
	},
	{
		Name:        "state_id",
		Type:        core.ConnectionTypeString,
		Label:       "State ID",
		Placeholder: "Workflow state UUID (optional, defaults to team backlog)",
	},
	{
		Name:        "label_ids",
		Type:        core.ConnectionTypeString,
		Label:       "Label IDs",
		Placeholder: "Comma-separated label UUIDs (optional)",
	},
	{
		Name:        "project_id",
		Type:        core.ConnectionTypeString,
		Label:       "Project ID",
		Placeholder: "Project UUID (optional)",
	},
	{
		Name:        "estimate",
		Type:        core.ConnectionTypeInteger,
		Label:       "Estimate",
		Placeholder: "Story points (optional)",
	},
	{
		Name:        "due_date",
		Type:        core.ConnectionTypeString,
		Label:       "Due Date",
		Placeholder: "YYYY-MM-DD (optional)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "issue_id", Type: core.ConnectionTypeString, Label: "Issue ID"},
	{Name: "identifier", Type: core.ConnectionTypeString, Label: "Identifier"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	teamID, err := linear.RequiredString("team_id", inputs)
	if err != nil {
		return nil, err
	}

	title, err := linear.RequiredString("title", inputs)
	if err != nil {
		return nil, err
	}

	vars := map[string]interface{}{
		"teamId": teamID,
		"title":  title,
	}

	if v := linear.OptionalString("description", inputs); v != "" {
		vars["description"] = v
	}
	if v := linear.OptionalString("priority", inputs); v != "" {
		var p int
		fmt.Sscanf(v, "%d", &p)
		vars["priority"] = p
	}
	if v := linear.OptionalString("assignee_id", inputs); v != "" {
		vars["assigneeId"] = v
	}
	if v := linear.OptionalString("state_id", inputs); v != "" {
		vars["stateId"] = v
	}
	if v := linear.OptionalString("project_id", inputs); v != "" {
		vars["projectId"] = v
	}
	if v := linear.OptionalString("due_date", inputs); v != "" {
		vars["dueDate"] = v
	}
	if conn := core.FindConnection("estimate", inputs); conn != nil && conn.Number() != nil {
		vars["estimate"] = *conn.Number()
	}
	if v := linear.OptionalString("label_ids", inputs); v != "" {
		var ids []string
		for _, id := range splitCSV(v) {
			if id != "" {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			vars["labelIds"] = ids
		}
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `mutation IssueCreate($input: IssueCreateInput!) {
			issueCreate(input: $input) {
				success
				issue {
					id
					identifier
					url
				}
			}
		}`,
		Variables: map[string]interface{}{"input": vars},
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to create issue: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		IssueCreate struct {
			Success bool `json:"success"`
			Issue   struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
				URL        string `json:"url"`
			} `json:"issue"`
		} `json:"issueCreate"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	issue := result.IssueCreate.Issue
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created %s: %s — %s", issue.Identifier, title, issue.URL),
		"issue_id":    issue.ID,
		"identifier":  issue.Identifier,
		"url":         issue.URL,
		"success":     result.IssueCreate.Success,
		"error":       "",
	}, nil
}

func splitCSV(s string) []string {
	var parts []string
	for _, p := range []byte(s) {
		if p == ',' {
			parts = append(parts, "")
		}
	}
	// Simple split by comma
	start := 0
	result := []string{}
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			v := s[start:i]
			// Trim spaces
			for len(v) > 0 && v[0] == ' ' {
				v = v[1:]
			}
			for len(v) > 0 && v[len(v)-1] == ' ' {
				v = v[:len(v)-1]
			}
			if v != "" {
				result = append(result, v)
			}
			start = i + 1
		}
	}
	_ = parts
	return result
}
