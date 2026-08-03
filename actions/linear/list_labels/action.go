package linear_list_labels

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
	Name         = "List Labels"
	Description  = "List issue labels with their UUIDs. Use this to get the label UUIDs needed by update_issue."
	Website      = "https://www.flomation.co"
	Icon         = "linear+list"
	Date         = "03/08/2026"
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
	{
		Name:        "team_id",
		Type:        core.ConnectionTypeString,
		Label:       "Team ID or Key (optional)",
		Placeholder: "UUID or team key (e.g. FLO). Blank = all workspace labels.",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "labels", Type: core.ConnectionTypeObject, Label: "Labels"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	// team_id is optional. Blank lists every label in the workspace (which is
	// what you want when resolving a label name to its UUID). A team key (e.g.
	// "FLO") is resolved to its UUID; a UUID is used directly.
	var variables map[string]interface{}
	if teamID := linear.OptionalString("team_id", inputs); teamID != "" {
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
		variables = map[string]interface{}{
			"filter": map[string]interface{}{
				"team": map[string]interface{}{"id": map[string]interface{}{"eq": teamID}},
			},
		}
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query ListLabels($filter: IssueLabelFilter) {
			issueLabels(first: 250, filter: $filter) {
				nodes {
					id
					name
					color
					isGroup
					parent { id name }
					team { id key }
				}
			}
		}`,
		Variables: variables,
	})
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	var result struct {
		IssueLabels struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"issueLabels"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	type labelRow struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Parent struct {
			Name string `json:"name"`
		} `json:"parent"`
		Team struct {
			Key string `json:"key"`
		} `json:"team"`
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d label(s):\n\n", len(result.IssueLabels.Nodes))
	var parsed []interface{}
	for _, raw := range result.IssueLabels.Nodes {
		var row labelRow
		if err := json.Unmarshal(raw, &row); err == nil {
			name := row.Name
			if row.Parent.Name != "" {
				name = row.Parent.Name + " › " + name
			}
			scope := "workspace"
			if row.Team.Key != "" {
				scope = row.Team.Key
			}
			fmt.Fprintf(&sb, "• %s [%s] {id:%s}\n", name, scope, row.ID)
		}
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		parsed = append(parsed, generic)
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"labels":      parsed,
		"count":       len(result.IssueLabels.Nodes),
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
