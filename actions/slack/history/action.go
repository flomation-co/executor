// Package slack_history is an AI agent tool for reading Slack channel history.
package slack_history

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Slack Channel History"
	Description  = "Read recent messages from a Slack channel. Uses conversations.history API."
	Website      = "https://www.flomation.co"
	Icon         = "slack+clock"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{Name: "bot_token", Type: core.ConnectionTypeSecret, Label: "Bot Token", Placeholder: "xoxb-...", Required: true},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID", Placeholder: "C01ABC2DEF3", Required: true},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Number of messages (default 20, max 100)"},
	{Name: "oldest", Type: core.ConnectionTypeString, Label: "Oldest timestamp (Unix ts or ISO date, e.g. 2026-04-28)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "messages", Type: core.ConnectionTypeObject, Label: "Messages (JSON array)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Message count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	botToken := str("bot_token", inputs)
	if botToken == "" {
		return nil, fmt.Errorf("bot_token is required")
	}
	channelID := str("channel_id", inputs)
	if channelID == "" {
		return nil, fmt.Errorf("channel_id is required")
	}

	limit := str("limit", inputs)
	if limit == "" {
		limit = "20"
	}

	params := url.Values{}
	params.Set("channel", channelID)
	params.Set("limit", limit)

	if oldest := str("oldest", inputs); oldest != "" {
		// Support ISO date format — convert to Unix timestamp
		if t, err := time.Parse("2006-01-02", oldest); err == nil {
			params.Set("oldest", fmt.Sprintf("%d", t.Unix()))
		} else if t, err := time.Parse(time.RFC3339, oldest); err == nil {
			params.Set("oldest", fmt.Sprintf("%d", t.Unix()))
		} else {
			// Assume it's already a Unix timestamp
			params.Set("oldest", oldest)
		}
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, slackAPIBase+"/conversations.history?"+params.Encode(), nil)
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
		return fail("Slack conversations.history failed: " + errMsg)
	}

	messages, _ := result["messages"].([]interface{})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Channel %s — %d message(s):\n\n", channelID, len(messages)))

	// Messages come newest-first from Slack; reverse for chronological reading
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}
		user, _ := msg["user"].(string)
		text, _ := msg["text"].(string)
		ts, _ := msg["ts"].(string)
		threadTS, _ := msg["thread_ts"].(string)
		replyCount, _ := msg["reply_count"].(float64)

		if len(text) > 300 {
			text = text[:300] + "..."
		}

		threadInfo := ""
		if threadTS != "" && replyCount > 0 {
			threadInfo = fmt.Sprintf(" [thread: %.0f replies]", replyCount)
		}

		sb.WriteString(fmt.Sprintf("[%s] <@%s>: %s%s\n", ts, user, text, threadInfo))
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"messages":    messages,
		"count":       int64(len(messages)),
		"success":     true,
		"error":       "",
	}, nil
}

func fail(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{"tool_result": msg, "messages": nil, "count": int64(0), "success": false, "error": msg}, nil
}

func str(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}
