// Package slack_search is an AI agent tool for searching Slack messages.
package slack_search

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
	Name         = "Slack Search Messages"
	Description  = "Search Slack messages. Requires a user token (xoxp-), not a bot token. Supports modifiers: from:user, in:channel, before:date, after:date"
	Website      = "https://www.flomation.co"
	Icon         = "slack+magnifying-glass"
	Date         = "20/04/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{Name: "user_token", Type: core.ConnectionTypeString, Label: "User Token (required — search needs a user token, not bot token)", Placeholder: "xoxp-...", Required: true},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search query. Supports Slack search modifiers: from:@user, in:#channel, before:2026-04-20, after:2026-04-01, has:link", Required: true},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Max results to return (default 10, max 100)"},
	{Name: "sort", Type: core.ConnectionTypeString, Label: "Sort order: score (relevance) or timestamp (recent first). Default: score"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "messages", Type: core.ConnectionTypeObject, Label: "Matching messages"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total matches"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// search.messages requires a user token (xoxp-), not a bot token (xoxb-).
	userToken := str("user_token", inputs)
	if userToken == "" {
		return nil, fmt.Errorf("user_token is required (search.messages needs a user token, not a bot token)")
	}
	query := str("query", inputs)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	count := str("count", inputs)
	if count == "" {
		count = "10"
	}
	sort := str("sort", inputs)
	if sort == "" {
		sort = "score"
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("count", count)
	params.Set("sort", sort)
	params.Set("sort_dir", "desc")

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, slackAPIBase+"/search.messages?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail("Slack API error: " + err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fail("Failed to parse Slack response")
	}
	if ok, _ := result["ok"].(bool); !ok {
		errMsg, _ := result["error"].(string)
		return fail("Slack search failed: " + errMsg)
	}

	msgs := result["messages"]
	var matches []interface{}
	var total float64
	if msgMap, ok := msgs.(map[string]interface{}); ok {
		total, _ = msgMap["total"].(float64)
		if m, ok := msgMap["matches"].([]interface{}); ok {
			matches = m
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d messages matching %q.\n", int(total), query))
	for i, m := range matches {
		if i >= 10 {
			sb.WriteString(fmt.Sprintf("... and %d more\n", len(matches)-10))
			break
		}
		if msg, ok := m.(map[string]interface{}); ok {
			user, _ := msg["username"].(string)
			text, _ := msg["text"].(string)
			ts, _ := msg["ts"].(string)
			ch, _ := msg["channel"].(map[string]interface{})
			chName := ""
			if ch != nil {
				chName, _ = ch["name"].(string)
			}
			if len(text) > 120 {
				text = text[:120] + "..."
			}
			sb.WriteString(fmt.Sprintf("• [%s] %s in #%s: %s\n", ts, user, chName, text))
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"messages":    matches,
		"total":       int(total),
		"success":     true,
		"error":       "",
	}, nil
}

func fail(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{"tool_result": msg, "messages": nil, "total": 0, "success": false, "error": msg}, nil
}

func str(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}
