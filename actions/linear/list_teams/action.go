package linear_list_teams

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Teams"
	Description  = "List all teams in the Linear workspace with their workflow states"
	Website      = "https://www.flomation.co"
	Icon         = "users"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	linear.AuthInputs[0],
}

var Outputs = [...]core.Connection{
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
			"success": false,
			"error":   err.Error(),
		}, nil
	}

	var result struct {
		Teams struct {
			Nodes []interface{} `json:"nodes"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return map[string]interface{}{
		"teams":   result.Teams.Nodes,
		"count":   len(result.Teams.Nodes),
		"success": true,
		"error":   "",
	}, nil
}