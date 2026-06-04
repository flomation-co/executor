// Package reply sends a reply to a Teams conversation activity using the
// Bot Framework REST API. Requires the service_url and activity_id from
// the incoming trigger data.
package reply

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Reply to Teams Message"
	Description  = "Send a reply to a Teams conversation using the Bot Framework"
	Website      = "https://www.flomation.co"
	Icon         = "microsoft+reply"
	Date         = "05/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "message", Type: core.ConnectionTypeText, Label: "Message", Required: true},
	{Name: "service_url", Type: core.ConnectionTypeString, Label: "Service URL", Required: true, Placeholder: "${service_url}"},
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID", Required: true, Placeholder: "${channel_id}"},
	{Name: "activity_id", Type: core.ConnectionTypeString, Label: "Activity ID", Placeholder: "${activity_id}"},
	{Name: "app_id", Type: core.ConnectionTypeString, Label: "Bot App ID", Required: true, Placeholder: "${secrets.TEAMS_APP_ID}"},
	{Name: "app_password", Type: core.ConnectionTypeString, Label: "Bot App Password", Required: true, Placeholder: "${secrets.TEAMS_APP_PASSWORD}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	message := microsoft.OptStr("message", inputs)
	serviceURL := microsoft.OptStr("service_url", inputs)
	conversationID := microsoft.OptStr("conversation_id", inputs)
	activityID := microsoft.OptStr("activity_id", inputs)
	appID := microsoft.OptStr("app_id", inputs)
	appPassword := microsoft.OptStr("app_password", inputs)

	if message == "" {
		return microsoft.ErrorResult("message is required")
	}
	if serviceURL == "" {
		return microsoft.ErrorResult("service_url is required")
	}
	if conversationID == "" {
		return microsoft.ErrorResult("conversation_id is required")
	}
	if appID == "" || appPassword == "" {
		return microsoft.ErrorResult("app_id and app_password are required")
	}

	// Get a Bot Framework token
	token, err := getBotToken(appID, appPassword)
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("unable to get bot token: %v", err))
	}

	// Build the reply
	reply := map[string]interface{}{
		"type": "message",
		"from": map[string]string{
			"id": appID,
		},
		"conversation": map[string]string{
			"id": conversationID,
		},
		"text":       message,
		"textFormat": "markdown",
	}

	body, err := json.Marshal(reply)
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("marshal reply: %v", err))
	}

	// Choose endpoint: reply to activity (threaded) or post new message
	svcURL := serviceURL
	if !strings.HasSuffix(svcURL, "/") {
		svcURL += "/"
	}

	var endpoint string
	if activityID != "" {
		endpoint = fmt.Sprintf("%sv3/conversations/%s/activities/%s", svcURL, conversationID, activityID)
	} else {
		endpoint = fmt.Sprintf("%sv3/conversations/%s/activities", svcURL, conversationID)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("create request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("send reply: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return microsoft.ErrorResult(fmt.Sprintf("Bot Framework returned %d: %s", resp.StatusCode, microsoft.TruncateBody(respBody)))
	}

	// Parse response for message ID
	var result struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &result)

	return map[string]interface{}{
		"tool_result": "Message sent to Teams conversation",
		"message_id":  result.ID,
		"success":     true,
		"error":       "",
	}, nil
}

func getBotToken(appID, appPassword string) (string, error) {
	data := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s&scope=https%%3A%%2F%%2Fapi.botframework.com%%2F.default",
		appID, appPassword)

	resp, err := http.Post(
		"https://login.microsoftonline.com/botframework.com/oauth2/v2.0/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data),
	)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.AccessToken, nil
}
