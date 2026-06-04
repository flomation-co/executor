package slack

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
	Name         = "Send Slack Message"
	Description  = "Send a message to a Slack channel via the Bot API with mrkdwn formatting and optional Block Kit layouts"
	Website      = "https://www.flomation.co"
	Icon         = "slack"
	Date         = "03/04/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{
		Name:        "bot_token",
		Type:        core.ConnectionTypeString,
		Label:       "Bot Token",
		Placeholder: "xoxb-...",
		Required:    true,
	},
	{
		Name:        "channel_id",
		Type:        core.ConnectionTypeString,
		Label:       "Channel ID",
		Placeholder: "${channel_id}",
		Required:    true,
	},
	{
		Name:        "message",
		Type:        core.ConnectionTypeText,
		Label:       "Message",
		Placeholder: "Hello from Flomation! Use *bold*, _italic_, `code`",
		Required:    true,
	},
	{
		Name:        "thread_id",
		Type:        core.ConnectionTypeString,
		Label:       "Thread ID",
		Placeholder: "${thread_id}",
	},
	{
		Name:        "blocks",
		Type:        core.ConnectionTypeText,
		Label:       "Block Kit JSON",
		Placeholder: `[{"type":"section","text":{"type":"mrkdwn","text":"*Rich* message"}}]`,
	},
	{
		Name:        "attachments",
		Type:        core.ConnectionTypeText,
		Label:       "Attachments JSON",
		Placeholder: `[{"color":"#36a64f","text":"Attachment text"}]`,
	},
	{
		Name:        "unfurl_links",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Unfurl Links",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Message Timestamp"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	botTokenConn := core.FindConnection("bot_token", inputs)
	if botTokenConn == nil || botTokenConn.String() == nil || *botTokenConn.String() == "" {
		return nil, fmt.Errorf("bot_token is required")
	}
	botToken := *botTokenConn.String()

	// Accept both canonical "channel_id" and legacy "chat_id" for
	// cross-provider compatibility (Telegram flows use chat_id).
	channelConn := core.FindConnection("channel_id", inputs)
	if channelConn == nil || channelConn.String() == nil || *channelConn.String() == "" {
		channelConn = core.FindConnection("chat_id", inputs)
	}
	if channelConn == nil || channelConn.String() == nil || *channelConn.String() == "" {
		return nil, fmt.Errorf("channel_id is required")
	}
	channelID := *channelConn.String()

	messageConn := core.FindConnection("message", inputs)
	if messageConn == nil || messageConn.String() == nil || *messageConn.String() == "" {
		// Empty message — the AI likely communicated via tools and has no
		// final text to send. Skip gracefully instead of failing.
		return map[string]interface{}{
			"tool_result": "no message to send (empty response)",
			"message_id":  "",
			"success":     true,
			"error":       "",
		}, nil
	}
	message := *messageConn.String()

	// Accept both canonical "thread_id" and Slack-specific "thread_ts".
	threadTS := ""
	threadConn := core.FindConnection("thread_id", inputs)
	if threadConn == nil || threadConn.String() == nil || *threadConn.String() == "" {
		threadConn = core.FindConnection("thread_ts", inputs)
	}
	if threadConn != nil && threadConn.String() != nil {
		threadTS = *threadConn.String()
	}

	// Build the payload with mrkdwn enabled by default.
	payload := map[string]interface{}{
		"channel": channelID,
		"text":    message,
		"mrkdwn":  true,
	}
	if threadTS != "" {
		payload["thread_ts"] = threadTS
	}

	// Block Kit: parse and include blocks array if provided.
	if blocksConn := core.FindConnection("blocks", inputs); blocksConn != nil && blocksConn.String() != nil {
		blocksJSON := strings.TrimSpace(*blocksConn.String())
		if blocksJSON != "" {
			var blocks []interface{}
			if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
				log.WithFields(log.Fields{
					"error": err,
					"raw":   blocksJSON[:min(len(blocksJSON), 100)],
				}).Warn("failed to parse blocks JSON — sending as plain text")
			} else {
				payload["blocks"] = blocks
			}
		}
	}

	// Attachments: secondary content below the main message.
	if attachConn := core.FindConnection("attachments", inputs); attachConn != nil && attachConn.String() != nil {
		attachJSON := strings.TrimSpace(*attachConn.String())
		if attachJSON != "" {
			var attachments []interface{}
			if err := json.Unmarshal([]byte(attachJSON), &attachments); err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Warn("failed to parse attachments JSON — skipping")
			} else {
				payload["attachments"] = attachments
			}
		}
	}

	// Unfurl links control.
	if unfurlConn := core.FindConnection("unfurl_links", inputs); unfurlConn != nil && unfurlConn.Boolean() != nil {
		payload["unfurl_links"] = *unfurlConn.Boolean()
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
			"timestamp": "",
			"success":   false,
			"error":     err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var result struct {
		OK    bool   `json:"ok"`
		TS    string `json:"ts"`
		Error string `json:"error"`
	}
	json.Unmarshal(respBody, &result)

	if !result.OK {
		return map[string]interface{}{
			"timestamp": "",
			"success":   false,
			"error":     result.Error,
		}, nil
	}

	return map[string]interface{}{
		"timestamp": result.TS,
		"success":   true,
		"error":     "",
	}, nil
}
