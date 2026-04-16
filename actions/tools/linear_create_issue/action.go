// Package linear_create_issue is a tool wrapper for the linear/create_issue
// action, making it available as an AI agent tool.
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
	Name         = "Linear Create Issue"
	Description  = "Create a new issue in Linear. Requires team_id (use linear_list_teams to find it). Returns the issue identifier and URL. Always confirm the team before creating."
	Website      = "https://www.flomation.co"
	Icon    = "linear"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Linear API key", Required: true},
	{Name: "team_id", Type: core.ConnectionTypeString, Label: "Team UUID (from linear_list_teams)", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Issue title", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Markdown description of the issue"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "Priority: 0=none, 1=urgent, 2=high, 3=medium, 4=low"},
	{Name: "assignee_id", Type: core.ConnectionTypeString, Label: "Assignee user UUID"},
	{Name: "state_id", Type: core.ConnectionTypeString, Label: "Workflow state UUID"},
	{Name: "label_ids", Type: core.ConnectionTypeString, Label: "Comma-separated label UUIDs"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project UUID"},
	{Name: "due_date", Type: core.ConnectionTypeString, Label: "Due date in YYYY-MM-DD format"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary for the AI"},
	{Name: "issue_id", Type: core.ConnectionTypeString, Label: "Issue UUID"},
	{Name: "identifier", Type: core.ConnectionTypeString, Label: "Human-readable identifier (e.g. ENG-123)"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Web URL"},
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
	if v := linear.OptionalString("label_ids", inputs); v != "" {
		var ids []string
		for _, part := range splitTrimmed(v) {
			if part != "" {
				ids = append(ids, part)
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
				issue { id identifier url }
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
		"tool_result": fmt.Sprintf("Created issue %s: %s — %s", issue.Identifier, title, issue.URL),
		"issue_id":    issue.ID,
		"identifier":  issue.Identifier,
		"url":         issue.URL,
		"success":     result.IssueCreate.Success,
		"error":       "",
	}, nil
}

func splitTrimmed(s string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			v := s[start:i]
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
	return result
}
