// Package slack_rich_message is an AI agent tool for sending rich Slack
// messages with Block Kit layouts, attachments, and mrkdwn formatting.
//
// The AI constructs blocks dynamically based on the conversation context.
// Simple text replies should use the standard messaging/slack action via
// the Response handle — this tool is for when the agent needs structured
// layouts (sections with fields, images, buttons, dividers, context blocks).
package slack_rich_message

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Slack Rich Message"
	Description  = "Send a rich Slack message with Block Kit layouts. Use this when you need structured formatting: " +
		"sections with fields, images, buttons, dividers, or context blocks. For simple text replies, " +
		"use the normal response instead. Blocks use Slack mrkdwn (*bold*, _italic_, ~strike~, `code`)."
	Website = "https://www.flomation.co"
	Icon    = "slack"
	Date    = "15/04/2026"
	Type    = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{
		Name:     "bot_token",
		Type:     core.ConnectionTypeString,
		Label:    "Slack bot token (xoxb-...)",
		Required: true,
	},
	{
		Name:     "channel_id",
		Type:     core.ConnectionTypeString,
		Label:    "Channel or DM ID to send to",
		Required: true,
	},
	{
		Name:     "text",
		Type:     core.ConnectionTypeText,
		Label:    "Fallback text shown in notifications and accessibility (plain summary of the message)",
		Required: true,
	},
	{
		Name: "blocks",
		Type: core.ConnectionTypeText,
		Label: "Block Kit blocks as a JSON array. Common block types: " +
			"section (with mrkdwn text or fields), divider, image, context, header, actions (buttons). " +
			"Example: [{\"type\":\"header\",\"text\":{\"type\":\"plain_text\",\"text\":\"Report\"}},{\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"*Status:* All clear\"}}]",
		Required: true,
	},
	{
		Name:  "thread_ts",
		Type:  core.ConnectionTypeString,
		Label: "Thread timestamp to reply in (optional)",
	},
	{
		Name:  "attachments",
		Type:  core.ConnectionTypeText,
		Label: "Legacy attachments JSON array — colour bars, fields, footers (optional)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary for the AI"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Message timestamp"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	botToken := requireString("bot_token", inputs)
	if botToken == "" {
		return nil, fmt.Errorf("bot_token is required")
	}
	channelID := requireString("channel_id", inputs)
	if channelID == "" {
		return nil, fmt.Errorf("channel_id is required")
	}
	text := requireString("text", inputs)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	blocksRaw := optionalString("blocks", inputs)
	if blocksRaw == "" {
		return nil, fmt.Errorf("blocks is required")
	}

	// Parse blocks JSON.
	var blocks []interface{}
	if err := json.Unmarshal([]byte(blocksRaw), &blocks); err != nil {
		// Try to recover: the AI sometimes wraps in markdown fences.
		cleaned := strings.TrimSpace(blocksRaw)
		if strings.HasPrefix(cleaned, "```") {
			if idx := strings.Index(cleaned[3:], "\n"); idx != -1 {
				cleaned = cleaned[3+idx+1:]
			}
			cleaned = strings.TrimSuffix(strings.TrimSpace(cleaned), "```")
			cleaned = strings.TrimSpace(cleaned)
		}
		if err2 := json.Unmarshal([]byte(cleaned), &blocks); err2 != nil {
			return map[string]interface{}{
				"tool_result": fmt.Sprintf("Invalid blocks JSON: %s", err),
				"success":     false,
				"error":       err.Error(),
			}, nil
		}
	}

	payload := map[string]interface{}{
		"channel": channelID,
		"text":    text,
		"mrkdwn":  true,
		"blocks":  blocks,
	}

	if threadTS := optionalString("thread_ts", inputs); threadTS != "" {
		payload["thread_ts"] = threadTS
	}

	// Parse optional attachments.
	if attachRaw := optionalString("attachments", inputs); attachRaw != "" {
		var attachments []interface{}
		if err := json.Unmarshal([]byte(attachRaw), &attachments); err != nil {
			log.WithError(err).Warn("failed to parse attachments JSON — skipping")
		} else {
			payload["attachments"] = attachments
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, slackAPIBase+"/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to send: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var result struct {
		OK    bool   `json:"ok"`
		TS    string `json:"ts"`
		Error string `json:"error"`
	}
	_ = json.Unmarshal(respBody, &result)

	if !result.OK {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Slack API error: %s", result.Error),
			"timestamp":   "",
			"success":     false,
			"error":       result.Error,
		}, nil
	}

	blockCount := len(blocks)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Rich message sent with %d blocks", blockCount),
		"timestamp":   result.TS,
		"success":     true,
		"error":       "",
	}, nil
}

func requireString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

func optionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}
