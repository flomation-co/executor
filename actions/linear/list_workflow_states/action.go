package linear_list_workflow_states

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
	Name         = "List Workflow States"
	Description  = "List workflow states (e.g. Todo, In Progress, Done, Cancelled) for a team. Use this to get state UUIDs needed by update_issue."
	Website      = "https://www.flomation.co"
	Icon         = "linear"
	Date         = "17/04/2026"
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
		Label:       "Team ID (UUID, from list_teams)",
		Placeholder: "Team UUID",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "states", Type: core.ConnectionTypeObject, Label: "Workflow States"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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
		return map[string]interface{}{
			"tool_result": "team_id is required — use list_teams first to get team UUIDs",
			"success":     false,
			"error":       "team_id is required",
		}, nil
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query ListWorkflowStates($teamId: String!) {
			workflowStates(filter: { team: { id: { eq: $teamId } } }, first: 50) {
				nodes {
					id
					name
					type
					position
					color
				}
			}
		}`,
		Variables: map[string]interface{}{
			"teamId": teamID,
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
		WorkflowStates struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"workflowStates"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	type stateRow struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		Type     string  `json:"type"`
		Position float64 `json:"position"`
		Color    string  `json:"color"`
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d workflow state(s):\n\n", len(result.WorkflowStates.Nodes))
	var parsed []interface{}
	for _, raw := range result.WorkflowStates.Nodes {
		var row stateRow
		if err := json.Unmarshal(raw, &row); err == nil {
			fmt.Fprintf(&sb, "• %s (type: %s) {id:%s}\n", row.Name, row.Type, row.ID)
		}
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		parsed = append(parsed, generic)
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"states":      parsed,
		"count":       len(result.WorkflowStates.Nodes),
		"success":     true,
		"error":       "",
	}, nil
}