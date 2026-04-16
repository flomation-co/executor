// Package linear_get_issue is a tool wrapper for the linear/get_issue
// action, making it available as an AI agent tool.
package linear_get_issue

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
	Name         = "Linear Get Issue"
	Description  = "Fetch details of a single Linear issue by its UUID or human-readable identifier (e.g. ENG-123). Returns title, description, state, priority, assignee, and more."
	Website      = "https://www.flomation.co"
	Icon    = "linear"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Linear API key", Required: true},
	{Name: "issue_id", Type: core.ConnectionTypeString, Label: "Issue UUID or identifier (e.g. ENG-123)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Issue details for the AI"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Full issue data"},
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

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query GetIssue($id: String!) {
			issue(id: $id) {
				id identifier title description url
				priority priorityLabel dueDate estimate
				createdAt updatedAt
				state { id name }
				assignee { id name email }
				team { id name key }
				project { id name }
				labels { nodes { id name } }
			}
		}`,
		Variables: map[string]interface{}{"id": issueID},
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to fetch issue: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		Issue struct {
			ID            string  `json:"id"`
			Identifier    string  `json:"identifier"`
			Title         string  `json:"title"`
			Description   string  `json:"description"`
			URL           string  `json:"url"`
			PriorityLabel string  `json:"priorityLabel"`
			DueDate       *string `json:"dueDate"`
			State         *struct{ Name string } `json:"state"`
			Assignee      *struct{ Name string } `json:"assignee"`
			Team          *struct{ Name, Key string } `json:"team"`
			Labels        struct {
				Nodes []struct{ Name string } `json:"nodes"`
			} `json:"labels"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	i := result.Issue
	var parts []string
	parts = append(parts, fmt.Sprintf("%s: %s", i.Identifier, i.Title))
	if i.State != nil {
		parts = append(parts, fmt.Sprintf("State: %s", i.State.Name))
	}
	parts = append(parts, fmt.Sprintf("Priority: %s", i.PriorityLabel))
	if i.Assignee != nil {
		parts = append(parts, fmt.Sprintf("Assignee: %s", i.Assignee.Name))
	} else {
		parts = append(parts, "Assignee: unassigned")
	}
	if i.Team != nil {
		parts = append(parts, fmt.Sprintf("Team: %s (%s)", i.Team.Name, i.Team.Key))
	}
	if i.DueDate != nil {
		parts = append(parts, fmt.Sprintf("Due: %s", *i.DueDate))
	}
	if len(i.Labels.Nodes) > 0 {
		var names []string
		for _, l := range i.Labels.Nodes {
			names = append(names, l.Name)
		}
		parts = append(parts, fmt.Sprintf("Labels: %s", strings.Join(names, ", ")))
	}
	if i.Description != "" {
		desc := i.Description
		if len(desc) > 500 {
			desc = desc[:500] + "…"
		}
		parts = append(parts, fmt.Sprintf("Description: %s", desc))
	}
	parts = append(parts, fmt.Sprintf("URL: %s", i.URL))

	// Full result as JSON object
	fullJSON, _ := json.Marshal(result.Issue)
	var fullObj interface{}
	_ = json.Unmarshal(fullJSON, &fullObj)

	return map[string]interface{}{
		"tool_result": strings.Join(parts, "\n"),
		"result":      fullObj,
		"success":     true,
		"error":       "",
	}, nil
}
