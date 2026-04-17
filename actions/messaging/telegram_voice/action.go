// Package telegram_voice sends a voice message via the Telegram Bot API.
// It can take pre-encoded OGG audio (base64) directly, or convert text to
// speech using ElevenLabs first and then send the resulting audio.
package telegram_voice

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Telegram Voice"
	Description  = "Send a voice message via Telegram. Provide audio as base64 OGG, or text to auto-convert via ElevenLabs TTS."
	Website      = "https://www.flomation.co"
	Icon         = "paper-plane"
	Date         = "18/04/2026"
	Type         = core.ActionTypeAction

	telegramAPIBase = "https://api.telegram.org"
	elevenlabsAPI   = "https://api.elevenlabs.io/v1"
)

var Inputs = [...]core.Connection{
	{
		Name:        "bot_token",
		Type:        core.ConnectionTypeString,
		Label:       "Telegram Bot Token",
		Placeholder: "123456:ABC-DEF...",
		Required:    true,
	},
	{
		Name:        "channel_id",
		Type:        core.ConnectionTypeString,
		Label:       "Chat/Channel ID",
		Placeholder: "${channel_id}",
		Required:    true,
	},
	{
		Name:  "message",
		Type:  core.ConnectionTypeText,
		Label: "Text to speak (uses ElevenLabs TTS). Ignored if audio_base64 is provided.",
	},
	{
		Name:  "audio_base64",
		Type:  core.ConnectionTypeString,
		Label: "Pre-encoded audio (base64, OGG format). If provided, sent directly.",
	},
	{
		Name:  "elevenlabs_api_key",
		Type:  core.ConnectionTypeString,
		Label: "ElevenLabs API Key (required for text-to-speech)",
	},
	{
		Name:  "voice_id",
		Type:  core.ConnectionTypeString,
		Label: "ElevenLabs Voice ID (default: Rachel)",
		Placeholder: "21m00Tcm4TlvDq8ikWAM",
	},
	{
		Name:  "model_id",
		Type:  core.ConnectionTypeString,
		Label: "ElevenLabs Model",
		Options: []core.ConnectionOption{
			{Name: "Multilingual v2", Value: "eleven_multilingual_v2"},
			{Name: "Turbo v2.5", Value: "eleven_turbo_v2_5"},
		},
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

	audioB64 := optionalString("audio_base64", inputs)
	message := optionalString("message", inputs)
	caption := optionalString("caption", inputs)

	var audioData []byte

	if audioB64 != "" {
		// Use pre-encoded audio directly
		var err error
		audioData, err = base64.StdEncoding.DecodeString(audioB64)
		if err != nil {
			return errResult(fmt.Sprintf("Failed to decode audio_base64: %v", err))
		}
	} else if message != "" {
		// Convert text to speech via ElevenLabs
		elAPIKey := optionalString("elevenlabs_api_key", inputs)
		if elAPIKey == "" {
			return errResult("elevenlabs_api_key is required when using text-to-speech (no audio_base64 provided)")
		}

		voiceID := optionalString("voice_id", inputs)
		if voiceID == "" {
			voiceID = "21m00Tcm4TlvDq8ikWAM" // Rachel (default)
		}

		modelID := optionalString("model_id", inputs)
		if modelID == "" {
			modelID = "eleven_multilingual_v2"
		}

		var err error
		audioData, err = textToSpeech(flow, elAPIKey, message, voiceID, modelID)
		if err != nil {
			return errResult(fmt.Sprintf("ElevenLabs TTS failed: %v", err))
		}
	} else {
		return errResult("Either message (for TTS) or audio_base64 (pre-encoded) is required")
	}

	// Send via Telegram sendVoice
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

func textToSpeech(flow *core.Flow, apiKey, text, voiceID, modelID string) ([]byte, error) {
	payload, _ := json.Marshal(map[string]interface{}{
		"text":     text,
		"model_id": modelID,
	})

	// Request OGG/OPUS format — Telegram's native voice format
	endpoint := fmt.Sprintf("%s/text-to-speech/%s?output_format=mp3_44100_128",
		elevenlabsAPI, voiceID)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 50<<20))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ElevenLabs API returned %d: %s", resp.StatusCode, string(data[:min(len(data), 500)]))
	}

	return data, nil
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
