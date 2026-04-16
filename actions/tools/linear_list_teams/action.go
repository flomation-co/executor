// Package linear_list_teams is a tool wrapper for the linear/list_teams
// action, making it available as an AI agent tool.
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
	Name         = "Linear List Teams"
	Description  = "List all teams in the Linear workspace. Returns team IDs, names, workflow states, labels, and members. Call this first to get team_id for creating issues."
	Website      = "https://www.flomation.co"
	Icon    = "linear"
	Date         = "15/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeString, Label: "Linear API key", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Team list summary for the AI"},
	{Name: "teams", Type: core.ConnectionTypeObject, Label: "Full team data array"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Number of teams"},
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
					id name key description
					states { nodes { id name type } }
					labels { nodes { id name } }
					members { nodes { id name email } }
				}
			}
		}`,
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to list teams: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		Teams struct {
			Nodes []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Key         string `json:"key"`
				Description string `json:"description"`
				States      struct {
					Nodes []struct{ ID, Name, Type string } `json:"nodes"`
				} `json:"states"`
				Labels struct {
					Nodes []struct{ ID, Name string } `json:"nodes"`
				} `json:"labels"`
				Members struct {
					Nodes []struct{ ID, Name, Email string } `json:"nodes"`
				} `json:"members"`
			} `json:"nodes"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var teamSummaries []string
	for _, team := range result.Teams.Nodes {
		summary := fmt.Sprintf("Team: %s (%s) — ID: %s", team.Name, team.Key, team.ID)
		if len(team.States.Nodes) > 0 {
			var stateNames []string
			for _, s := range team.States.Nodes {
				stateNames = append(stateNames, fmt.Sprintf("%s [%s]", s.Name, s.ID))
			}
			summary += fmt.Sprintf("\n  States: %s", strings.Join(stateNames, ", "))
		}
		if len(team.Members.Nodes) > 0 {
			var memberNames []string
			for _, m := range team.Members.Nodes {
				memberNames = append(memberNames, fmt.Sprintf("%s [%s]", m.Name, m.ID))
			}
			summary += fmt.Sprintf("\n  Members: %s", strings.Join(memberNames, ", "))
		}
		teamSummaries = append(teamSummaries, summary)
	}

	fullJSON, _ := json.Marshal(result.Teams.Nodes)
	var fullObj interface{}
	_ = json.Unmarshal(fullJSON, &fullObj)

	return map[string]interface{}{
		"tool_result": strings.Join(teamSummaries, "\n\n"),
		"teams":       fullObj,
		"count":       len(result.Teams.Nodes),
		"success":     true,
		"error":       "",
	}, nil
}