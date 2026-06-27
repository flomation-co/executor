// Package telegram_video sends a video via the Telegram Bot API.
// Preferred over send_document for MP4/MOV uploads because Telegram
// inlines the video player and computes a thumbnail server-side.
package telegram_video

import (
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	telegram_common "flomation.app/automate/executor/actions/messaging/telegram"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Telegram Video"
	Description  = "Send a video (MP4, with or without audio track) via Telegram. THE CORRECT CHOICE for output from Gemini Video / Veo and any other video file. Accepts a flo:blob token or base64. Do NOT use send_voice or send_audio for video files — those strip the visual track."
	Website      = "https://www.flomation.co"
	Icon         = "telegram+video"
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
		Label: "Video to send (flo:blob: token from an upstream action)",
	},
	{
		Name:  "file_base64",
		Type:  core.ConnectionTypeString,
		Label: "Video bytes as base64 (alternative to file_blob)",
	},
	{
		Name:  "caption",
		Type:  core.ConnectionTypeString,
		Label: "Caption text shown beneath the video",
	},
	{
		Name:  "duration",
		Type:  core.ConnectionTypeInteger,
		Label: "Video duration in seconds (optional)",
	},
	{
		Name:  "width",
		Type:  core.ConnectionTypeInteger,
		Label: "Video width (optional)",
	},
	{
		Name:  "height",
		Type:  core.ConnectionTypeInteger,
		Label: "Video height (optional)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "message_id", Type: core.ConnectionTypeInteger, Label: "Message ID"},
	{Name: "video_size_bytes", Type: core.ConnectionTypeInteger, Label: "Video size (bytes)"},
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
	duration := telegram_common.OptionalString("duration", inputs)
	width := telegram_common.OptionalString("width", inputs)
	height := telegram_common.OptionalString("height", inputs)

	data, err := telegram_common.ResolveFileBytes(flow, fileBlob, fileB64)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to resolve video: %v", err))
	}

	extra := map[string]string{}
	if duration != "" && duration != "0" {
		extra["duration"] = duration
	}
	if width != "" && width != "0" {
		extra["width"] = width
	}
	if height != "" && height != "0" {
		extra["height"] = height
	}

	msgID, err := telegram_common.SendMultipartFile(
		flow, botToken, channelID, "sendVideo", "video", "video.mp4",
		data, caption, extra,
	)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to send video: %v", err))
	}

	durationDisp := ""
	if duration != "" && duration != "0" {
		if _, perr := strconv.Atoi(duration); perr == nil {
			durationDisp = fmt.Sprintf(" (%ss)", duration)
		}
	}
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Video sent%s, %d bytes", durationDisp, len(data)),
		"message_id":       msgID,
		"video_size_bytes": len(data),
		"success":          true,
		"error":            "",
	}, nil
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":      msg,
		"message_id":       0,
		"video_size_bytes": 0,
		"success":          false,
		"error":            msg,
	}, nil
}
