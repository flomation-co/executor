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
	Name         = "Update Issue"
	Description  = "Update an existing Linear issue's title, description, status, priority, or assignee"
	Website      = "https://www.flomation.co"
	Icon         = "pencil"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	linear.AuthInputs[0],
	{
		Name:        "issue_id",
		Type:        core.ConnectionTypeString,
		Label:       "Issue ID",
		Placeholder: "Issue UUID",
		Required:    true,
	},
	{
		Name:        "title",
		Type:        core.ConnectionTypeString,
		Label:       "Title",
		Placeholder: "New title (optional)",
	},
	{
		Name:        "description",
		Type:        core.ConnectionTypeText,
		Label:       "Description",
		Placeholder: "New description (optional)",
	},
	{
		Name:  "priority",
		Type:  core.ConnectionTypeString,
		Label: "Priority",
		Options: []core.ConnectionOption{
			{Name: "(unchanged)", Value: ""},
			{Name: "No Priority", Value: "0"},
			{Name: "Urgent", Value: "1"},
			{Name: "High", Value: "2"},
			{Name: "Medium", Value: "3"},
			{Name: "Low", Value: "4"},
		},
	},
	{
		Name:        "state_id",
		Type:        core.ConnectionTypeString,
		Label:       "State ID",
		Placeholder: "Workflow state UUID (optional)",
	},
	{
		Name:        "assignee_id",
		Type:        core.ConnectionTypeString,
		Label:       "Assignee ID",
		Placeholder: "User UUID (optional)",
	},
	{
		Name:        "project_id",
		Type:        core.ConnectionTypeString,
		Label:       "Project ID",
		Placeholder: "Project UUID (optional)",
	},
	{
		Name:        "due_date",
		Type:        core.ConnectionTypeString,
		Label:       "Due Date",
		Placeholder: "YYYY-MM-DD (optional)",
	},
}

var Outputs = [...]core.Connection{
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
	if v := linear.OptionalString("project_id", inputs); v != "" {
		update["projectId"] = v
	}
	if v := linear.OptionalString("due_date", inputs); v != "" {
		update["dueDate"] = v
	}

	if len(update) == 0 {
		return map[string]interface{}{
			"success": false,
			"error":   "at least one field to update is required",
		}, nil
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `mutation IssueUpdate($id: String!, $input: IssueUpdateInput!) {
			issueUpdate(id: $id, input: $input) {
				success
				issue {
					id
					identifier
					url
				}
			}
		}`,
		Variables: map[string]interface{}{
			"id":    issueID,
			"input": update,
		},
	})
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
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

	return map[string]interface{}{
		"issue_id":   result.IssueUpdate.Issue.ID,
		"identifier": result.IssueUpdate.Issue.Identifier,
		"url":        result.IssueUpdate.Issue.URL,
		"success":    result.IssueUpdate.Success,
		"error":      "",
	}, nil
}
