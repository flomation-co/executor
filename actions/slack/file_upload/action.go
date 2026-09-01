// Package slack_file_upload is an AI agent tool for uploading files to Slack.
// Uses the modern files.getUploadURLExternal -> upload -> files.completeUploadExternal flow.
//
// Accepts three flavours of payload, chosen in this priority order:
//
//  1. file_blob  — a flo:blob:... reference (typically populated by
//     the AI tool loop from an upstream action that produced binary
//     output: TTS audio, generated image, chart render, etc.)
//  2. file_base64 — base64-encoded bytes (manual / legacy wiring)
//  3. content    — plain text (the original M0 behaviour, kept for
//     code snippets / CSV / log paste flows)
//
// Exactly one is required; file_blob wins when multiple are set.
package slack_file_upload

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Slack File Upload"
	Description  = "Upload a file to a Slack channel. Accepts a flo:blob token, base64, or text."
	Website      = "https://www.flomation.co"
	Icon         = "slack+arrow-up"
	Date         = "21/06/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{Name: "bot_token", Type: core.ConnectionTypeSecret, Label: "Bot Token", Placeholder: "xoxb-...", Required: true},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID", Placeholder: "${channel_id}", Required: true},
	{Name: "file_blob", Type: core.ConnectionTypeString, Label: "File to upload (flo:blob: token from an upstream action)"},
	{Name: "file_base64", Type: core.ConnectionTypeString, Label: "File bytes as base64 (alternative to file_blob)"},
	{Name: "content", Type: core.ConnectionTypeText, Label: "File content as text (for code snippets, CSV, logs, etc.)"},
	{Name: "filename", Type: core.ConnectionTypeString,
		Label: "Filename including extension, e.g. report.csv (optional — defaults to the uploaded file's own name)"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Display title for the file in Slack (optional, defaults to filename)"},
	{Name: "initial_comment", Type: core.ConnectionTypeString, Label: "Message to post alongside the file (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "file_id", Type: core.ConnectionTypeString, Label: "Slack file ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	botToken := str("bot_token", inputs)
	if botToken == "" {
		return nil, fmt.Errorf("bot_token is required")
	}
	channelID := str("channel_id", inputs)
	if channelID == "" {
		channelID = str("chat_id", inputs)
	}
	if channelID == "" {
		return nil, fmt.Errorf("channel_id is required")
	}

	fileBlob := str("file_blob", inputs)
	fileB64 := str("file_base64", inputs)
	content := str("content", inputs)

	contentBytes, mimeType, err := resolveFileBytes(flow, fileBlob, fileB64, content)
	if err != nil {
		return fail(err.Error())
	}

	// Slack picks a preview renderer from the filename extension, so a screenshot
	// that arrives with no name (or the historical "file.bin" default) shows as an
	// unpreviewable octet-stream. deriveFilename respects a caller-supplied,
	// well-extensioned name but otherwise derives the extension from the resolved
	// MIME type (a flo:blob: token carries type=image/png; a flo:file: ref carries
	// its path extension) with a byte-sniff fallback.
	filename := deriveFilename(str("filename", inputs), fileBlob, content != "", mimeType, contentBytes)
	title := str("title", inputs)
	if title == "" {
		title = filename
	}
	initialComment := str("initial_comment", inputs)

	// Step 1: Get upload URL.
	params := url.Values{}
	params.Set("filename", filename)
	params.Set("length", fmt.Sprintf("%d", len(contentBytes)))

	getURLReq, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet,
		slackAPIBase+"/files.getUploadURLExternal?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create getUploadURL request: %w", err)
	}
	getURLReq.Header.Set("Authorization", "Bearer "+botToken)

	getURLResp, err := http.DefaultClient.Do(getURLReq)
	if err != nil {
		return fail("Failed to get upload URL: " + err.Error())
	}
	defer func() { _ = getURLResp.Body.Close() }()
	getURLBody, _ := io.ReadAll(io.LimitReader(getURLResp.Body, 8*1024))

	var urlResult map[string]interface{}
	if err := json.Unmarshal(getURLBody, &urlResult); err != nil {
		return fail("Failed to parse upload URL response")
	}
	if ok, _ := urlResult["ok"].(bool); !ok {
		errMsg, _ := urlResult["error"].(string)
		return fail("Get upload URL failed: " + errMsg)
	}

	uploadURL, _ := urlResult["upload_url"].(string)
	fileID, _ := urlResult["file_id"].(string)

	// Step 2: Upload file content.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fail("Failed to create multipart form: " + err.Error())
	}
	if _, err := part.Write(contentBytes); err != nil {
		return fail("Failed to write file content: " + err.Error())
	}
	_ = writer.Close()

	uploadReq, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, uploadURL, &buf)
	if err != nil {
		return fail("Failed to create upload request: " + err.Error())
	}
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+botToken)

	uploadResp, err := http.DefaultClient.Do(uploadReq)
	if err != nil {
		return fail("File upload failed: " + err.Error())
	}
	defer func() { _ = uploadResp.Body.Close() }()
	_, _ = io.ReadAll(uploadResp.Body)

	if uploadResp.StatusCode >= 300 {
		return fail(fmt.Sprintf("Upload returned HTTP %d", uploadResp.StatusCode))
	}

	// Step 3: Complete upload and share to channel.
	completePayload := map[string]interface{}{
		"files":      []map[string]interface{}{{"id": fileID, "title": title}},
		"channel_id": channelID,
	}
	if initialComment != "" {
		completePayload["initial_comment"] = initialComment
	}
	completeBody, _ := json.Marshal(completePayload)

	completeReq, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost,
		slackAPIBase+"/files.completeUploadExternal", bytes.NewReader(completeBody))
	if err != nil {
		return fail("Failed to create complete request: " + err.Error())
	}
	completeReq.Header.Set("Authorization", "Bearer "+botToken)
	completeReq.Header.Set("Content-Type", "application/json; charset=utf-8")

	completeResp, err := http.DefaultClient.Do(completeReq)
	if err != nil {
		return fail("Complete upload failed: " + err.Error())
	}
	defer func() { _ = completeResp.Body.Close() }()
	completeRespBody, _ := io.ReadAll(io.LimitReader(completeResp.Body, 8*1024))

	var completeResult map[string]interface{}
	if err := json.Unmarshal(completeRespBody, &completeResult); err != nil {
		return fail("Failed to parse complete response")
	}
	if ok, _ := completeResult["ok"].(bool); !ok {
		errMsg, _ := completeResult["error"].(string)
		return fail("Complete upload failed: " + errMsg)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("File %q uploaded to channel %s", filename, channelID),
		"file_id":     fileID,
		"success":     true,
		"error":       "",
	}, nil
}

func fail(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{"tool_result": msg, "file_id": "", "success": false, "error": msg}, nil
}

func str(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

// resolveFileBytes picks the bytes to upload from whichever input
// was supplied, in priority order: file_blob → file_base64 → content.
// At least one must be non-empty; otherwise we error.
//
// The file_blob path mirrors the Telegram M4 actions: if the value
// is still a flo:blob: token (e.g. the AI tool loop's
// DetokeniseInputs failed to resolve, or the action was invoked
// outside the loop), fetch the bytes from the blob store directly.
// Otherwise the string already IS the raw bytes (DetokeniseInputs
// resolved on the way in; Go strings carry arbitrary bytes fine).
func resolveFileBytes(flow *core.Flow, fileBlob, fileBase64, content string) ([]byte, string, error) {
	if fileBlob != "" {
		// flo:file: and flo:blob: both resolve via ResolveToBytes, which also
		// hands back the MIME type (the workspace file's detected type, or the
		// blob token's type= hint) — used to name the upload correctly.
		if core.IsFileRef(fileBlob) || core.IsBlobToken(fileBlob) {
			data, mimeType, err := flow.ResolveToBytes(fileBlob)
			if err != nil {
				return nil, "", fmt.Errorf("resolve file_blob: %w", err)
			}
			return data, mimeType, nil
		}
		// Already-resolved raw bytes (DetokeniseInputs ran on the way in).
		return []byte(fileBlob), "", nil
	}
	if fileBase64 != "" {
		data, err := base64.StdEncoding.DecodeString(fileBase64)
		if err == nil {
			return data, "", nil
		}
		data, err = base64.URLEncoding.DecodeString(fileBase64)
		if err != nil {
			return nil, "", fmt.Errorf("decode file_base64: %w", err)
		}
		return data, "", nil
	}
	if content != "" {
		return []byte(content), "text/plain", nil
	}
	return nil, "", fmt.Errorf("one of file_blob, file_base64, or content is required")
}

// deriveFilename returns a Slack upload filename with an extension Slack can use
// to choose a preview renderer. It keeps a caller-supplied name that already has
// a usable, specific extension (the AI often knows "report.csv"), but when the
// name is missing or generic — empty, no extension, or the historical
// "file.bin"/"file.txt" default — it falls back to the name the file reference
// carried from wherever it came from, and failing that derives the extension
// from the resolved MIME type, sniffing the bytes as a last resort. isText
// nudges an unknown payload towards .txt rather than .bin.
func deriveFilename(name, ref string, isText bool, mimeType string, data []byte) string {
	if n := usableName(name); n != "" {
		return n
	}
	// The reference often knows the real name: a download keeps what the
	// server called it, and an asset keeps what was uploaded.
	if n := usableName(core.FilenameForRef(ref)); n != "" {
		return n
	}
	base := strings.TrimSuffix(strings.TrimSpace(name), path.Ext(strings.TrimSpace(name)))
	if base == "" || strings.EqualFold(base, "file") {
		base = "file"
	}
	return base + extensionForContent(isText, mimeType, data)
}

// usableName returns name when it already carries a specific extension, else "".
// ".bin" is treated as no extension: it was this action's historical default
// and tells Slack nothing about how to preview the file.
func usableName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if ext := path.Ext(name); ext != "" && !strings.EqualFold(ext, ".bin") {
		return core.SanitiseFilename(name)
	}
	return ""
}

// extensionForContent maps a MIME type (or, failing that, sniffed bytes) to a
// file extension, including the leading dot.
func extensionForContent(isText bool, mimeType string, data []byte) string {
	mt := strings.TrimSpace(mimeType)
	if mt == "" {
		mt = http.DetectContentType(data) // e.g. image/png, video/mp4, application/pdf
	}
	if i := strings.IndexByte(mt, ';'); i >= 0 { // drop "; charset=..."
		mt = strings.TrimSpace(mt[:i])
	}
	switch strings.ToLower(mt) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "application/pdf":
		return ".pdf"
	case "text/csv":
		return ".csv"
	case "application/json":
		return ".json"
	case "application/zip":
		return ".zip"
	case "text/plain":
		return ".txt"
	}
	if exts, _ := mime.ExtensionsByType(mt); len(exts) > 0 {
		return exts[0]
	}
	if isText {
		return ".txt"
	}
	return ".bin"
}
