// Package linear_update_issue is a tool wrapper for the linear/update_issue
// action, making it available as an AI agent tool.
package linear_update_issue

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Linear Update Issue"
	Description  = "Update an existing Linear issue. Pass the issue UUID and any fields to change. Only provided fields are updated; omitted fields are left unchanged."
	Website      = "https://www.flomation.co"
	Icon         = "pencil"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Linear API key", Required: true},
	{Name: "issue_id", Type: core.ConnectionTypeString, Label: "Issue UUID to update", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "New title"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "New markdown description"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "New priority: 0=none, 1=urgent, 2=high, 3=medium, 4=low"},
	{Name: "state_id", Type: core.ConnectionTypeString, Label: "New workflow state UUID"},
	{Name: "assignee_id", Type: core.ConnectionTypeString, Label: "New assignee user UUID"},
	{Name: "due_date", Type: core.ConnectionTypeString, Label: "New due date in YYYY-MM-DD format"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary for the AI"},
	{Name: "identifier", Type: core.ConnectionTypeString, Label: "Issue identifier"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Web URL"},
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

	update := map[string]interface{}{}
	if v := linear.OptionalString("title", inputs); v != "" {
		update["title"] = v
	}
	if v := linear.OptionalString("description", inputs); v != "" {
		update["description"] = v
	}
	if v := linear.OptionalString("priority", inputs); v != "" {
		var p int
		fmt.Sscanf(v, "%d", &p)
		update["priority"] = p
	}
	if v := linear.OptionalString("state_id", inputs); v != "" {
		update["stateId"] = v
	}
	if v := linear.OptionalString("assignee_id", inputs); v != "" {
		update["assigneeId"] = v
	}
	if v := linear.OptionalString("due_date", inputs); v != "" {
		update["dueDate"] = v
	}

	if len(update) == 0 {
		return map[string]interface{}{
			"tool_result": "No fields provided to update",
			"success":     false,
			"error":       "at least one field to update is required",
		}, nil
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `mutation IssueUpdate($id: String!, $input: IssueUpdateInput!) {
			issueUpdate(id: $id, input: $input) {
				success
				issue { id identifier url }
			}
		}`,
		Variables: map[string]interface{}{"id": issueID, "input": update},
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to update issue: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		IssueUpdate struct {
			Success bool `json:"success"`
			Issue   struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
				URL        string `json:"url"`
			} `json:"issue"`
		} `json:"issueUpdate"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	issue := result.IssueUpdate.Issue
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated issue %s — %s", issue.Identifier, issue.URL),
		"identifier":  issue.Identifier,
		"url":         issue.URL,
		"success":     result.IssueUpdate.Success,
		"error":       "",
	}, nil
}
