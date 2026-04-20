// Package linear_list_users lists Linear workspace members.
package linear_list_users

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
	Name         = "List Users"
	Description  = "List Linear workspace members. Returns user IDs, names, emails, and active status."
	Website      = "https://www.flomation.co"
	Icon         = "linear"
	Date         = "20/04/2026"
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
		Name:  "search",
		Type:  core.ConnectionTypeString,
		Label: "Optional: filter by name or email",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "users", Type: core.ConnectionTypeObject, Label: "User list"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Number of users"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	search := linear.OptionalString("search", inputs)

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query {
			users {
				nodes {
					id
					name
					displayName
					email
					active
					admin
					avatarUrl
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
		Users struct {
			Nodes []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
				Email       string `json:"email"`
				Active      bool   `json:"active"`
				Admin       bool   `json:"admin"`
			} `json:"nodes"`
		} `json:"users"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	searchLower := strings.ToLower(search)
	var filtered []interface{}
	var sb strings.Builder

	for _, u := range result.Users.Nodes {
		if !u.Active {
			continue
		}
		if search != "" {
			if !strings.Contains(strings.ToLower(u.Name), searchLower) &&
				!strings.Contains(strings.ToLower(u.Email), searchLower) &&
				!strings.Contains(strings.ToLower(u.DisplayName), searchLower) {
				continue
			}
		}

		role := "member"
		if u.Admin {
			role = "admin"
		}
		fmt.Fprintf(&sb, "• %s <%s> (id:%s, %s)\n", u.Name, u.Email, u.ID, role)

		raw, _ := json.Marshal(u)
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		filtered = append(filtered, generic)
	}

	header := fmt.Sprintf("Found %d user(s)", len(filtered))
	if search != "" {
		header += fmt.Sprintf(" matching %q", search)
	}
	summary := header + ":\n" + sb.String()

	return map[string]interface{}{
		"tool_result": summary,
		"users":       filtered,
		"count":       len(filtered),
		"success":     true,
		"error":       "",
	}, nil
}
