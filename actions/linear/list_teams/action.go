package linear_list_teams

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
	Name         = "List Teams"
	Description  = "List all Linear teams with workflow states, labels, and members. Call first to get team_id."
	Website      = "https://www.flomation.co"
	Icon         = "linear+user-group"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "teams", Type: core.ConnectionTypeObject, Label: "Teams"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query ListTeams {
			teams {
				nodes {
					id
					name
					key
					description
					states {
						nodes {
							id
							name
							type
							position
						}
					}
					labels {
						nodes {
							id
							name
						}
					}
					members {
						nodes {
							id
							name
							email
						}
					}
				}
			}
		}`,
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		Teams struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	type teamRow struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Name string `json:"name"`
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d team(s):\n\n", len(result.Teams.Nodes))
	var parsed []interface{}
	for _, raw := range result.Teams.Nodes {
		var row teamRow
		if err := json.Unmarshal(raw, &row); err == nil {
			fmt.Fprintf(&sb, "• %s (%s) {id:%s}\n", row.Name, row.Key, row.ID)
		}
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		parsed = append(parsed, generic)
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"teams":       parsed,
		"count":       len(result.Teams.Nodes),
		"success":     true,
		"error":       "",
	}, nil
}
