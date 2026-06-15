// Package slack_react is an AI agent tool for adding emoji reactions to Slack messages.
package slack_react

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Slack Add Reaction"
	Description  = "Add an emoji reaction to a Slack message. Use emoji name without colons."
	Website      = "https://www.flomation.co"
	Icon         = "slack+star"
	Date         = "28/04/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{Name: "bot_token", Type: core.ConnectionTypeSecret, Label: "Bot Token", Placeholder: "xoxb-...", Required: true},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID", Placeholder: "C01ABC2DEF3", Required: true},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Message Timestamp", Placeholder: "1234567890.123456", Required: true},
	{Name: "emoji", Type: core.ConnectionTypeString, Label: "Emoji Name (without colons)", Placeholder: "thumbsup", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	timestamp := str("timestamp", inputs)
	if timestamp == "" {
		return nil, fmt.Errorf("timestamp is required")
	}
	emoji := str("emoji", inputs)
	if emoji == "" {
		return nil, fmt.Errorf("emoji is required")
	}

	// Strip colons if the user included them
	emoji = strings.Trim(emoji, ":")

	payload, _ := json.Marshal(map[string]string{
		"channel":   channelID,
		"timestamp": timestamp,
		"name":      emoji,
	})

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, slackAPIBase+"/reactions.add", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail("Slack API error: " + err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fail("Failed to parse Slack response")
	}
	if ok, _ := result["ok"].(bool); !ok {
		errMsg, _ := result["error"].(string)
		if errMsg == "already_reacted" {
			return map[string]interface{}{
				"tool_result": fmt.Sprintf("Already reacted with :%s: on message %s", emoji, timestamp),
				"success":     true,
				"error":       "",
			}, nil
		}
		return fail("Slack reactions.add failed: " + errMsg)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Added :%s: reaction to message %s in %s", emoji, timestamp, channelID),
		"success":     true,
		"error":       "",
	}, nil
}

func fail(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{"tool_result": msg, "success": false, "error": msg}, nil
}

func str(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}
