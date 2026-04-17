package linear_update_issue

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
	Name         = "Update Issue"
	Description  = "Update an existing Linear issue. Pass issue UUID and fields to change. Omitted fields are unchanged."
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
		Label:       "State ID (UUID, optional — use state_name instead for convenience)",
		Placeholder: "Workflow state UUID",
	},
	{
		Name:        "state_name",
		Type:        core.ConnectionTypeString,
		Label:       "State Name (e.g. 'Cancelled', 'In Progress', 'Done' — resolves to UUID automatically)",
		Placeholder: "State name",
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
	} else if v := linear.OptionalString("state_name", inputs); v != "" {
		// Resolve state name to UUID by fetching the issue's team and
		// looking up the matching workflow state.
		stateID, err := resolveStateName(apiKey, issueID, v)
		if err != nil {
			return map[string]interface{}{
				"tool_result": fmt.Sprintf("Failed to resolve state '%s': %s", v, err),
				"success":     false,
				"error":       err.Error(),
			}, nil
		}
		update["stateId"] = stateID
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
			"tool_result": "No fields provided to update",
			"success":     false,
			"error":       "at least one field to update is required",
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
			"tool_result": fmt.Sprintf("Failed to update: %s", err),
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

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated %s — %s", result.IssueUpdate.Issue.Identifier, result.IssueUpdate.Issue.URL),
		"issue_id":    result.IssueUpdate.Issue.ID,
		"identifier":  result.IssueUpdate.Issue.Identifier,
		"url":         result.IssueUpdate.Issue.URL,
		"success":     result.IssueUpdate.Success,
		"error":       "",
	}, nil
}

// resolveStateName fetches the issue's team, then looks up the workflow
// state by name (case-insensitive) and returns its UUID.
func resolveStateName(apiKey, issueID, stateName string) (string, error) {
	// Step 1: get the issue's team ID
	issueResp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query GetIssueTeam($id: String!) {
			issue(id: $id) {
				team { id }
			}
		}`,
		Variables: map[string]interface{}{"id": issueID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch issue team: %w", err)
	}

	var issueResult struct {
		Issue struct {
			Team struct {
				ID string `json:"id"`
			} `json:"team"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(issueResp.Data, &issueResult); err != nil {
		return "", fmt.Errorf("failed to parse issue: %w", err)
	}
	teamID := issueResult.Issue.Team.ID
	if teamID == "" {
		return "", fmt.Errorf("could not determine team for issue %s", issueID)
	}

	// Step 2: list workflow states for the team
	statesResp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query ListStates($teamId: String!) {
			workflowStates(filter: { team: { id: { eq: $teamId } } }, first: 50) {
				nodes {
					id
					name
				}
			}
		}`,
		Variables: map[string]interface{}{"teamId": teamID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch workflow states: %w", err)
	}

	var statesResult struct {
		WorkflowStates struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"workflowStates"`
	}
	if err := json.Unmarshal(statesResp.Data, &statesResult); err != nil {
		return "", fmt.Errorf("failed to parse states: %w", err)
	}

	// Case-insensitive match
	for _, s := range statesResult.WorkflowStates.Nodes {
		if strings.EqualFold(s.Name, stateName) {
			return s.ID, nil
		}
	}

	// Build helpful error with available state names
	var names []string
	for _, s := range statesResult.WorkflowStates.Nodes {
		names = append(names, s.Name)
	}
	return "", fmt.Errorf("state '%s' not found. Available states: %s", stateName, strings.Join(names, ", "))
}
