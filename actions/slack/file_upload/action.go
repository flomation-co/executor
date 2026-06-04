// Package slack_file_upload is an AI agent tool for uploading files to Slack.
// Uses the modern files.getUploadURLExternal -> upload -> files.completeUploadExternal flow.
package slack_file_upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Slack File Upload"
	Description  = "Upload a file to a Slack channel. Supports text content or base64-encoded binary data"
	Website      = "https://www.flomation.co"
	Icon         = "slack+arrow-up"
	Date         = "20/04/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{Name: "bot_token", Type: core.ConnectionTypeString, Label: "Bot Token", Placeholder: "xoxb-...", Required: true},
	{Name: "channel_id", Type: core.ConnectionTypeString, Label: "Channel ID", Placeholder: "${channel_id}", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "File content as text (for code snippets, CSV, logs, etc.)", Required: true},
	{Name: "filename", Type: core.ConnectionTypeString, Label: "Filename including extension (e.g. report.csv, log.txt, data.json)", Required: true},
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
	content := str("content", inputs)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	filename := str("filename", inputs)
	if filename == "" {
		filename = "file.txt"
	}
	title := str("title", inputs)
	if title == "" {
		title = filename
	}
	initialComment := str("initial_comment", inputs)
	contentBytes := []byte(content)

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
