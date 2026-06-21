// Package telegram_common holds helpers shared across the
// Telegram outbound file actions (send_photo, send_document,
// send_video, send_audio). The original send_message / send_voice
// actions were written before this category had multiple file-type
// siblings, so they remain self-contained — extracting their
// shared bits would be churn for no behavioural change. Anything
// added here is for the file-action family only.
//
// The manifest generator deliberately skips packages that have no
// Execute function (see CLAUDE.md and cmd/manifest/manifest.go), so
// this file is invisible to the editor's action list — exactly the
// right behaviour for a helper module.
package telegram_common

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
	TelegramAPIBase = "https://api.telegram.org"
)

// ResolveFileBytes returns the bytes to upload, working from
// whichever input the caller supplied. Resolution order:
//
//  1. fileBlob non-empty AND IsBlobToken → fetch via flow.Blobs(). This
//     branch fires when the tool-loop's DetokeniseInputs failed to
//     resolve the token (e.g. blob TTL expired) or when an action is
//     invoked outside the AI loop with a literal token.
//  2. fileBlob non-empty and NOT a token → it's already the raw bytes
//     (DetokeniseInputs resolved the token to bytes-as-string before
//     Execute ran). Convert verbatim.
//  3. fileBase64 non-empty → base64-decode, try both standard and
//     URL-safe alphabets (matches send_voice's behaviour).
//  4. Neither set → ErrNoFile.
//
// The "as string carries arbitrary bytes" idiom is the same one
// audio_base64 uses on send_voice; Go strings are byte-safe.
func ResolveFileBytes(flow *core.Flow, fileBlob, fileBase64 string) ([]byte, error) {
	if fileBlob != "" {
		if core.IsBlobToken(fileBlob) {
			data, err := flow.Blobs().Get(fileBlob)
			if err != nil {
				return nil, fmt.Errorf("resolve file_blob: %w", err)
			}
			return data, nil
		}
		return []byte(fileBlob), nil
	}
	if fileBase64 != "" {
		data, err := base64.StdEncoding.DecodeString(fileBase64)
		if err == nil {
			return data, nil
		}
		data, err = base64.URLEncoding.DecodeString(fileBase64)
		if err != nil {
			return nil, fmt.Errorf("decode file_base64: %w", err)
		}
		return data, nil
	}
	return nil, ErrNoFile
}

// ErrNoFile signals the caller supplied neither file_blob nor
// file_base64. Sentinel so each action's Execute can give a
// consistent, action-specific error message.
var ErrNoFile = fmt.Errorf("no file provided (set file_blob or file_base64)")

// SendMultipartFile uploads bytes to a Telegram sendXxx endpoint
// using the standard multipart form-data shape Telegram expects.
// Shared across send_photo, send_document, send_video, send_audio —
// each just supplies a different endpoint and form-field name.
//
// endpoint is the API path suffix (e.g. "sendPhoto", "sendDocument").
// formField is the part name Telegram expects for the binary —
// "photo", "document", "video", "audio" respectively. filename is
// what the form-file boundary advertises; Telegram echoes it back
// in the file metadata and some clients use it to pick an icon.
//
// extraFields lets each action add type-specific metadata
// (duration, performer for audio; width/height for video) without
// the helper needing to know about every Telegram-side option.
func SendMultipartFile(
	flow *core.Flow,
	botToken, chatID, endpoint, formField, filename string,
	fileData []byte,
	caption string,
	extraFields map[string]string,
) (int64, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("chat_id", chatID)
	if caption != "" {
		_ = writer.WriteField("caption", caption)
	}
	for k, v := range extraFields {
		if v == "" {
			continue
		}
		_ = writer.WriteField(k, v)
	}

	part, err := writer.CreateFormFile(formField, filename)
	if err != nil {
		return 0, err
	}
	if _, err := part.Write(fileData); err != nil {
		return 0, err
	}
	if err := writer.Close(); err != nil {
		return 0, err
	}

	url := fmt.Sprintf("%s/bot%s/%s", TelegramAPIBase, botToken, endpoint)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, &body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("parse %s response: %w", endpoint, err)
	}
	if !result.OK {
		return 0, fmt.Errorf("%s failed: %s", endpoint, result.Description)
	}
	return result.Result.MessageID, nil
}

// OptionalString reads a Connection by name and returns its string
// value, or "" if absent/empty. Duplicated from send_voice rather
// than imported so this package stays standalone (no cross-action
// import path).
func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

// ValidateChannelID rejects unresolved template variables (the
// classic "${flow.channel_id}" -> "${flow.channel_id}" literal
// failure when the upstream context didn't populate the field).
// Mirrors the check send_voice does inline.
func ValidateChannelID(channelID string) error {
	if strings.HasPrefix(channelID, "${") || strings.HasPrefix(channelID, "#{") {
		return fmt.Errorf("channel_id contains an unresolved template variable: %q — try ${flow.channel_id}", channelID)
	}
	return nil
}
