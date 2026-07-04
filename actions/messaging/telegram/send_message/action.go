package telegram

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
	Name         = "Send Telegram Message"
	Description  = "Send a message via the Telegram Bot API"
	Website      = "https://www.flomation.co"
	Icon         = "paper-plane"
	Date         = "03/04/2026"
	Type         = core.ActionTypeAction

	telegramAPIBase = "https://api.telegram.org"
	maxMessageLen   = 4096
)

var Inputs = [...]core.Connection{
	{
		Name:        "bot_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Bot Token",
		Placeholder: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		Required:    true,
	},
	{
		Name:        "channel_id",
		Type:        core.ConnectionTypeString,
		Label:       "Channel ID",
		Placeholder: "${flow.channel_id}",
		Required:    true,
	},
	{
		Name:        "message",
		Type:        core.ConnectionTypeText,
		Label:       "Message",
		Placeholder: "Hello from Flomation!",
		Required:    true,
	},
	{
		Name:  "parse_mode",
		Type:  core.ConnectionTypeString,
		Label: "Parse Mode",
		Options: []core.ConnectionOption{
			{Name: "None", Value: ""},
			{Name: "HTML", Value: "HTML"},
			{Name: "MarkdownV2", Value: "MarkdownV2"},
		},
	},
	{
		Name:        "reply_markup",
		Type:        core.ConnectionTypeText,
		Label:       "Reply Markup (JSON)",
		Placeholder: `{"inline_keyboard":[[{"text":"Approve","callback_data":"yes"}]]}`,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "message_id", Type: core.ConnectionTypeInteger, Label: "Message ID"},
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
	// backwards compatibility with existing flows.
	chatIDConn := core.FindConnection("channel_id", inputs)
	if chatIDConn == nil || chatIDConn.String() == nil || *chatIDConn.String() == "" {
		chatIDConn = core.FindConnection("chat_id", inputs)
	}
	if chatIDConn == nil || chatIDConn.String() == nil || *chatIDConn.String() == "" {
		return nil, fmt.Errorf("channel_id is required")
	}
	chatID := *chatIDConn.String()

	// Guard against unresolved template variables ("${channel_id}",
	// "${chat_id}", "#{channel_id}", etc.) leaking through to Telegram's
	// API and being persisted as a literal string into agent_conversation.
	// These appear when a flow declares a placeholder using the wrong
	// namespace (e.g. ${channel_id} instead of ${flow.channel_id}) — the
	// substitution loop in flow.go silently leaves the literal in place
	// rather than erroring, so we must catch it here.
	if strings.HasPrefix(chatID, "${") || strings.HasPrefix(chatID, "#{") {
		return nil, fmt.Errorf("channel_id contains an unresolved template variable: %q — flow author likely used the wrong namespace (try ${flow.channel_id})", chatID)
	}

	messageConn := core.FindConnection("message", inputs)
	if messageConn == nil || messageConn.String() == nil || *messageConn.String() == "" {
		// Empty message — the AI likely communicated via tools and has no
		// final text to send. Skip gracefully instead of failing.
		return map[string]interface{}{
			"tool_result": "no message to send (empty response)",
			"message_id":  0,
			"success":     true,
			"error":       "",
		}, nil
	}
	message := *messageConn.String()

	if len(message) > maxMessageLen {
		message = message[:maxMessageLen]
	}

	parseModeConn := core.FindConnection("parse_mode", inputs)
	parseMode := ""
	if parseModeConn != nil && parseModeConn.String() != nil {
		parseMode = *parseModeConn.String()
	}

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    message,
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}

	// Optional inline keyboard / reply markup. Passed as a JSON string (e.g.
	// by the Human-in-the-Loop node) and forwarded as a nested object so
	// Telegram renders interactive buttons.
	if rm := core.FindConnection("reply_markup", inputs); rm != nil && rm.String() != nil && strings.TrimSpace(*rm.String()) != "" {
		var markup interface{}
		if err := json.Unmarshal([]byte(*rm.String()), &markup); err != nil {
			return nil, fmt.Errorf("reply_markup is not valid JSON: %w", err)
		}
		payload["reply_markup"] = markup
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, botToken)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]interface{}{
			"message_id": nil,
			"success":    false,
			"error":      err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(respBody, &result)

	if !result.OK {
		return map[string]interface{}{
			"message_id": nil,
			"success":    false,
			"error":      result.Description,
		}, nil
	}

	return map[string]interface{}{
		"message_id": result.Result.MessageID,
		"success":    true,
		"error":      "",
	}, nil
}
