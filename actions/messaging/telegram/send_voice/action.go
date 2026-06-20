// Package telegram_voice sends a voice message via the Telegram Bot API.
// Takes pre-encoded audio as base64 input — use a separate TTS action
// (e.g. elevenlabs/text_to_speech) upstream to convert text to audio first.
package telegram_voice

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Telegram Voice"
	Description  = "Send a voice message via Telegram. Wire an ElevenLabs TTS node upstream to convert text to audio."
	Website      = "https://www.flomation.co"
	Icon         = "telegram+microphone"
	Date         = "18/04/2026"
	Type         = core.ActionTypeAction

	telegramAPIBase = "https://api.telegram.org"
)

var Inputs = [...]core.Connection{
	{
		Name:        "bot_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Telegram Bot Token",
		Placeholder: "123456:ABC-DEF...",
		Required:    true,
	},
	{
		Name:        "channel_id",
		Type:        core.ConnectionTypeString,
		Label:       "Chat/Channel ID",
		Placeholder: "${flow.channel_id}",
		Required:    true,
	},
	{
		Name:        "audio_base64",
		Type:        core.ConnectionTypeString,
		Label:       "Audio data (base64). Wire from an upstream TTS or audio action.",
		Required:    true,
	},
	{
		Name:  "caption",
		Type:  core.ConnectionTypeString,
		Label: "Caption text shown alongside the voice message",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "message_id", Type: core.ConnectionTypeInteger, Label: "Message ID"},
	{Name: "audio_size_bytes", Type: core.ConnectionTypeInteger, Label: "Audio size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	botToken := optionalString("bot_token", inputs)
	if botToken == "" {
		return errResult("bot_token is required")
	}

	channelID := optionalString("channel_id", inputs)
	if channelID == "" {
		return errResult("channel_id is required")
	}
	// Reject unresolved template variables — they'd otherwise be persisted
	// as a literal string into agent_conversation.channel_id.
	if strings.HasPrefix(channelID, "${") || strings.HasPrefix(channelID, "#{") {
		return errResult(fmt.Sprintf("channel_id contains an unresolved template variable: %q — try ${flow.channel_id}", channelID))
	}

	audioB64 := optionalString("audio_base64", inputs)
	if audioB64 == "" || strings.HasPrefix(audioB64, "${") {
		// Empty or unresolved variable — the TTS upstream likely hasn't run
		// (or the subgraph was re-executed after cache clearance). Skip gracefully.
		return map[string]interface{}{
			"tool_result": "no audio to send (empty or unresolved audio_base64)",
			"message_id":  0,
			"success":     true,
			"error":       "",
		}, nil
	}

	caption := optionalString("caption", inputs)

	audioData, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		// Try URL-safe base64
		audioData, err = base64.URLEncoding.DecodeString(audioB64)
		if err != nil {
			return errResult(fmt.Sprintf("Failed to decode audio_base64: %v", err))
		}
	}

	msgID, err := sendVoice(flow, botToken, channelID, audioData, caption)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to send voice message: %v", err))
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Voice message sent (%d bytes)", len(audioData)),
		"message_id":       msgID,
		"audio_size_bytes": len(audioData),
		"success":          true,
		"error":            "",
	}, nil
}

func sendVoice(flow *core.Flow, botToken, chatID string, audioData []byte, caption string) (int64, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("chat_id", chatID)
	if caption != "" {
		_ = writer.WriteField("caption", caption)
	}

	part, err := writer.CreateFormFile("voice", "voice.mp3")
	if err != nil {
		return 0, err
	}
	if _, err := part.Write(audioData); err != nil {
		return 0, err
	}
	if err := writer.Close(); err != nil {
		return 0, err
	}

	url := fmt.Sprintf("%s/bot%s/sendVoice", telegramAPIBase, botToken)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, &body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("failed to parse sendVoice response: %w", err)
	}
	if !result.OK {
		return 0, fmt.Errorf("sendVoice failed: %s", result.Description)
	}

	return result.Result.MessageID, nil
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":      msg,
		"message_id":       0,
		"audio_size_bytes": 0,
		"success":          false,
		"error":            msg,
	}, nil
}

func optionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
