package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
		Type:        core.ConnectionTypeString,
		Label:       "Bot Token",
		Placeholder: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		Required:    true,
	},
	{
		Name:        "chat_id",
		Type:        core.ConnectionTypeString,
		Label:       "Chat ID",
		Placeholder: "12345678 or @channelname",
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
}

var Outputs = [...]core.Connection{
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

	chatIDConn := core.FindConnection("chat_id", inputs)
	if chatIDConn == nil || chatIDConn.String() == nil || *chatIDConn.String() == "" {
		return nil, fmt.Errorf("chat_id is required")
	}
	chatID := *chatIDConn.String()

	messageConn := core.FindConnection("message", inputs)
	if messageConn == nil || messageConn.String() == nil || *messageConn.String() == "" {
		return nil, fmt.Errorf("message is required")
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
	json.Unmarshal(respBody, &result)

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
