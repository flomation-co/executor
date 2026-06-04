// Package slack_thread is an AI agent tool for reading Slack thread replies.
package slack_thread

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
	Name         = "Slack Thread Replies"
	Description  = "Read replies in a Slack thread. Requires the channel ID and thread timestamp."
	Website      = "https://www.flomation.co"
	Icon         = "slack+comments"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{Name: "bot_token", Type: core.ConnectionTypeString, Label: "Bot Token", Placeholder: "xoxb-...", Required: true},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID", Placeholder: "C01ABC2DEF3", Required: true},
	{Name: "thread_ts", Type: core.ConnectionTypeString, Label: "Thread Timestamp", Placeholder: "1234567890.123456", Required: true},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Number of replies (default 50, max 200)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "messages", Type: core.ConnectionTypeObject, Label: "Thread messages (JSON array)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Reply count"},
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
	threadTS := str("thread_ts", inputs)
	if threadTS == "" {
		return nil, fmt.Errorf("thread_ts is required")
	}

	limit := str("limit", inputs)
	if limit == "" {
		limit = "50"
	}

	params := url.Values{}
	params.Set("channel", channelID)
	params.Set("ts", threadTS)
	params.Set("limit", limit)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, slackAPIBase+"/conversations.replies?"+params.Encode(), nil)
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
		return fail("Slack conversations.replies failed: " + errMsg)
	}

	messages, _ := result["messages"].([]interface{})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Thread in %s (ts: %s) — %d message(s):\n\n", channelID, threadTS, len(messages)))

	for _, m := range messages {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		user, _ := msg["user"].(string)
		text, _ := msg["text"].(string)
		ts, _ := msg["ts"].(string)

		if len(text) > 300 {
			text = text[:300] + "..."
		}

		sb.WriteString(fmt.Sprintf("[%s] <@%s>: %s\n", ts, user, text))
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
