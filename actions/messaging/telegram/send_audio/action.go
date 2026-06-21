// Package telegram_audio sends a music / audio file via the
// Telegram Bot API. Distinct from send_voice (voice notes appear in
// a special player); use this when sending a regular audio file the
// user might want to download or browse alongside title + performer
// metadata.
package telegram_audio

import (
	"fmt"

	core "flomation.app/automate/executor"
	telegram_common "flomation.app/automate/executor/actions/messaging/telegram"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Telegram Audio"
	Description  = "Send a music/audio file via Telegram. Use send_voice for voice notes."
	Website      = "https://www.flomation.co"
	Icon         = "telegram+music"
	Date         = "21/06/2026"
	Type         = core.ActionTypeAction
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
		Name:  "file_blob",
		Type:  core.ConnectionTypeString,
		Label: "Audio to send (flo:blob: token from an upstream action)",
	},
	{
		Name:  "file_base64",
		Type:  core.ConnectionTypeString,
		Label: "Audio bytes as base64 (alternative to file_blob)",
	},
	{
		Name:  "title",
		Type:  core.ConnectionTypeString,
		Label: "Track title (optional)",
	},
	{
		Name:  "performer",
		Type:  core.ConnectionTypeString,
		Label: "Artist / performer (optional)",
	},
	{
		Name:  "duration",
		Type:  core.ConnectionTypeInteger,
		Label: "Duration in seconds (optional)",
	},
	{
		Name:  "caption",
		Type:  core.ConnectionTypeString,
		Label: "Caption text shown alongside the audio",
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
	botToken := telegram_common.OptionalString("bot_token", inputs)
	if botToken == "" {
		return errResult("bot_token is required")
	}
	channelID := telegram_common.OptionalString("channel_id", inputs)
	if channelID == "" {
		return errResult("channel_id is required")
	}
	if err := telegram_common.ValidateChannelID(channelID); err != nil {
		return errResult(err.Error())
	}

	fileBlob := telegram_common.OptionalString("file_blob", inputs)
	fileB64 := telegram_common.OptionalString("file_base64", inputs)
	caption := telegram_common.OptionalString("caption", inputs)
	title := telegram_common.OptionalString("title", inputs)
	performer := telegram_common.OptionalString("performer", inputs)
	duration := telegram_common.OptionalString("duration", inputs)

	data, err := telegram_common.ResolveFileBytes(flow, fileBlob, fileB64)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to resolve audio: %v", err))
	}

	extra := map[string]string{}
	if title != "" {
		extra["title"] = title
	}
	if performer != "" {
		extra["performer"] = performer
	}
	if duration != "" && duration != "0" {
		extra["duration"] = duration
	}

	msgID, err := telegram_common.SendMultipartFile(
		flow, botToken, channelID, "sendAudio", "audio", "audio.mp3",
		data, caption, extra,
	)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to send audio: %v", err))
	}

	tag := ""
	if title != "" {
		tag = fmt.Sprintf(" '%s'", title)
	}
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Audio%s sent (%d bytes)", tag, len(data)),
		"message_id":       msgID,
		"audio_size_bytes": len(data),
		"success":          true,
		"error":            "",
	}, nil
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
