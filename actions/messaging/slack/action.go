package slack

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
	Name         = "Send Slack Message"
	Description  = "Send a message to a Slack channel via the Bot API"
	Website      = "https://www.flomation.co"
	Icon         = "hashtag"
	Date         = "03/04/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{
		Name:        "bot_token",
		Type:        core.ConnectionTypeString,
		Label:       "Bot Token",
		Placeholder: "xoxb-...",
		Required:    true,
	},
	{
		Name:        "channel_id",
		Type:        core.ConnectionTypeString,
		Label:       "Channel ID",
		Placeholder: "C01234ABCDE",
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
		Name:        "thread_ts",
		Type:        core.ConnectionTypeString,
		Label:       "Thread Timestamp",
		Placeholder: "Reply in thread (optional)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Message Timestamp"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	botTokenConn := core.FindConnection("bot_token", inputs)
	if botTokenConn == nil || botTokenConn.String() == nil || *botTokenConn.String() == "" {
		return nil, fmt.Errorf("bot_token is required")
	}
	botToken := *botTokenConn.String()

	channelConn := core.FindConnection("channel_id", inputs)
	if channelConn == nil || channelConn.String() == nil || *channelConn.String() == "" {
		return nil, fmt.Errorf("channel_id is required")
	}
	channelID := *channelConn.String()

	messageConn := core.FindConnection("message", inputs)
	if messageConn == nil || messageConn.String() == nil || *messageConn.String() == "" {
		return nil, fmt.Errorf("message is required")
	}
	message := *messageConn.String()

	threadTS := ""
	threadConn := core.FindConnection("thread_ts", inputs)
	if threadConn != nil && threadConn.String() != nil {
		threadTS = *threadConn.String()
	}

	payload := map[string]interface{}{
		"channel": channelID,
		"text":    message,
	}
	if threadTS != "" {
		payload["thread_ts"] = threadTS
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, slackAPIBase+"/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]interface{}{
			"timestamp": "",
			"success":   false,
			"error":     err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var result struct {
		OK    bool   `json:"ok"`
		TS    string `json:"ts"`
		Error string `json:"error"`
	}
	json.Unmarshal(respBody, &result)

	if !result.OK {
		return map[string]interface{}{
			"timestamp": "",
			"success":   false,
			"error":     result.Error,
		}, nil
	}

	return map[string]interface{}{
		"timestamp": result.TS,
		"success":   true,
		"error":     "",
	}, nil
}
