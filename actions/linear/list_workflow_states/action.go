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
		Label:       "Team ID or Key",
		Placeholder: "UUID or team key (e.g. FLO)",
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

	// If the user passed a team key (e.g. "FLO") instead of UUID, resolve it.
	if len(teamID) < 36 && !strings.Contains(teamID, "-") {
		resolved, resolveErr := resolveTeamKey(apiKey, teamID)
		if resolveErr != nil {
			return map[string]interface{}{
				"tool_result": fmt.Sprintf("Could not resolve team key %q: %s", teamID, resolveErr),
				"success":     false,
				"error":       resolveErr.Error(),
			}, nil
		}
		teamID = resolved
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query ListWorkflowStates($teamId: String!) {
			team(id: $teamId) {
				states {
					nodes {
						id
						name
						type
						position
						color
					}
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
		Team struct {
			States struct {
				Nodes []json.RawMessage `json:"nodes"`
			} `json:"states"`
		} `json:"team"`
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
	fmt.Fprintf(&sb, "Found %d workflow state(s):\n\n", len(result.Team.States.Nodes))
	var parsed []interface{}
	for _, raw := range result.Team.States.Nodes {
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
		"count":       len(result.Team.States.Nodes),
		"success":     true,
		"error":       "",
	}, nil
}

func resolveTeamKey(apiKey, key string) (string, error) {
	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query { teams { nodes { id key } } }`,
	})
	if err != nil {
		return "", err
	}
	var result struct {
		Teams struct {
			Nodes []struct {
				ID  string `json:"id"`
				Key string `json:"key"`
			} `json:"nodes"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}
	for _, t := range result.Teams.Nodes {
		if strings.EqualFold(t.Key, key) {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("team with key %q not found", key)
}