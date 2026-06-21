// Package telegram_photo sends an image via the Telegram Bot API.
// Accepts either a flo:blob:... reference (preferred — the AI tool
// loop passes blob tokens verbatim from upstream actions) or a
// base64-encoded image (legacy / manual wiring).
package telegram_photo

import (
	"fmt"

	core "flomation.app/automate/executor"
	telegram_common "flomation.app/automate/executor/actions/messaging/telegram"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Telegram Photo"
	Description  = "Send a photo via Telegram. Accepts a flo:blob image token or base64."
	Website      = "https://www.flomation.co"
	Icon         = "telegram+image"
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
		Label: "Image to send (flo:blob: token from an upstream action)",
	},
	{
		Name:  "file_base64",
		Type:  core.ConnectionTypeString,
		Label: "Image bytes as base64 (alternative to file_blob)",
	},
	{
		Name:  "caption",
		Type:  core.ConnectionTypeString,
		Label: "Caption text shown beneath the photo",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "message_id", Type: core.ConnectionTypeInteger, Label: "Message ID"},
	{Name: "photo_size_bytes", Type: core.ConnectionTypeInteger, Label: "Photo size (bytes)"},
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

	data, err := telegram_common.ResolveFileBytes(flow, fileBlob, fileB64)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to resolve photo: %v", err))
	}

	msgID, err := telegram_common.SendMultipartFile(
		flow, botToken, channelID, "sendPhoto", "photo", "photo.jpg",
		data, caption, nil,
	)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to send photo: %v", err))
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Photo sent (%d bytes)", len(data)),
		"message_id":       msgID,
		"photo_size_bytes": len(data),
		"success":          true,
		"error":            "",
	}, nil
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":      msg,
		"message_id":       0,
		"photo_size_bytes": 0,
		"success":          false,
		"error":            msg,
	}, nil
}
