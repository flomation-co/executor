// Package slack_channels is an AI agent tool for listing and searching Slack channels.
package slack_channels

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
	Name         = "Slack Channels"
	Description  = "List or search Slack channels. Returns channel names, IDs, topics, and member counts"
	Website      = "https://www.flomation.co"
	Icon         = "slack+list"
	Date         = "20/04/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{Name: "bot_token", Type: core.ConnectionTypeSecret, Label: "Bot Token", Placeholder: "xoxb-...", Required: true},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Optional filter: only return channels whose name contains this text"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Max channels to return (default 50, max 200)"},
	{Name: "include_private", Type: core.ConnectionTypeBoolean, Label: "Include private channels the bot is a member of (default false)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "channels", Type: core.ConnectionTypeObject, Label: "Channel list"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Number of channels returned"},
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

	includePrivate := false
	if conn := core.FindConnection("include_private", inputs); conn != nil {
		if b, ok := conn.Value.(bool); ok {
			includePrivate = b
		} else if s := conn.String(); s != nil && (*s == "true" || *s == "1") {
			includePrivate = true
		}
	}

	types := "public_channel"
	if includePrivate {
		types = "public_channel,private_channel"
	}

	params := url.Values{}
	params.Set("types", types)
	params.Set("limit", limit)
	params.Set("exclude_archived", "true")

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, slackAPIBase+"/conversations.list?"+params.Encode(), nil)
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

	channels, _ := result["channels"].([]interface{})
	searchLower := strings.ToLower(search)
	var filtered []interface{}
	for _, ch := range channels {
		if m, ok := ch.(map[string]interface{}); ok {
			name, _ := m["name"].(string)
			if search == "" || strings.Contains(strings.ToLower(name), searchLower) {
				filtered = append(filtered, ch)
			}
		}
	}

	var sb strings.Builder
	if search != "" {
		sb.WriteString(fmt.Sprintf("Found %d channels matching %q:\n", len(filtered), search))
	} else {
		sb.WriteString(fmt.Sprintf("Found %d channels:\n", len(filtered)))
	}
	for i, ch := range filtered {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf("... and %d more\n", len(filtered)-20))
			break
		}
		if m, ok := ch.(map[string]interface{}); ok {
			name, _ := m["name"].(string)
			id, _ := m["id"].(string)
			topic := ""
			if t, ok := m["topic"].(map[string]interface{}); ok {
				topic, _ = t["value"].(string)
			}
			members, _ := m["num_members"].(float64)
			line := fmt.Sprintf("• #%s (%s) — %d members", name, id, int(members))
			if topic != "" && len(topic) <= 60 {
				line += " — " + topic
			}
			sb.WriteString(line + "\n")
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"channels":    filtered,
		"count":       len(filtered),
		"success":     true,
		"error":       "",
	}, nil
}

func fail(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{"tool_result": msg, "channels": nil, "count": 0, "success": false, "error": msg}, nil
}

func str(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}
