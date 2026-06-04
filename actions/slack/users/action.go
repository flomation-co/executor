// Package slack_users is an AI agent tool for listing and searching Slack workspace members.
package slack_users

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Slack Users"
	Description  = "List or search Slack workspace members. Returns display names, real names, email addresses, and status"
	Website      = "https://www.flomation.co"
	Icon         = "slack"
	Date         = "20/04/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{Name: "bot_token", Type: core.ConnectionTypeString, Label: "Bot Token", Placeholder: "xoxb-...", Required: true},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Optional filter: only return users whose name or display name contains this text"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Max users to return (default 50, max 200)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "users", Type: core.ConnectionTypeObject, Label: "User list"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Number of users returned"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	botToken := str("bot_token", inputs)
	if botToken == "" {
		return nil, fmt.Errorf("bot_token is required")
	}
	search := str("search", inputs)
	limit := str("limit", inputs)
	if limit == "" {
		limit = "50"
	}

	params := url.Values{}
	params.Set("limit", limit)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, slackAPIBase+"/users.list?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail("Slack API error: " + err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fail("Failed to parse Slack response")
	}
	if ok, _ := result["ok"].(bool); !ok {
		errMsg, _ := result["error"].(string)
		return fail("Slack API error: " + errMsg)
	}

	members, _ := result["members"].([]interface{})
	searchLower := strings.ToLower(search)
	var filtered []interface{}
	for _, u := range members {
		m, ok := u.(map[string]interface{})
		if !ok {
			continue
		}
		if deleted, _ := m["deleted"].(bool); deleted {
			continue
		}
		if isBot, _ := m["is_bot"].(bool); isBot {
			continue
		}
		if search != "" {
			name, _ := m["real_name"].(string)
			displayName := ""
			if p, ok := m["profile"].(map[string]interface{}); ok {
				displayName, _ = p["display_name"].(string)
			}
			if !strings.Contains(strings.ToLower(name), searchLower) &&
				!strings.Contains(strings.ToLower(displayName), searchLower) {
				continue
			}
		}
		filtered = append(filtered, u)
	}

	var sb strings.Builder
	if search != "" {
		sb.WriteString(fmt.Sprintf("Found %d users matching %q:\n", len(filtered), search))
	} else {
		sb.WriteString(fmt.Sprintf("Found %d users:\n", len(filtered)))
	}
	for i, u := range filtered {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf("... and %d more\n", len(filtered)-20))
			break
		}
		if m, ok := u.(map[string]interface{}); ok {
			id, _ := m["id"].(string)
			realName, _ := m["real_name"].(string)
			displayName := ""
			email := ""
			statusText := ""
			if p, ok := m["profile"].(map[string]interface{}); ok {
				displayName, _ = p["display_name"].(string)
				email, _ = p["email"].(string)
				statusText, _ = p["status_text"].(string)
			}
			name := realName
			if displayName != "" && displayName != realName {
				name = fmt.Sprintf("%s (%s)", realName, displayName)
			}
			line := fmt.Sprintf("• %s [%s]", name, id)
			if email != "" {
				line += " — " + email
			}
			if statusText != "" {
				line += " — " + statusText
			}
			sb.WriteString(line + "\n")
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"users":       filtered,
		"count":       len(filtered),
		"success":     true,
		"error":       "",
	}, nil
}

func fail(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{"tool_result": msg, "users": nil, "count": 0, "success": false, "error": msg}, nil
}

func str(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}
