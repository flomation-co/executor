// Package channel_action sends channel-specific SDK actions like typing
// indicators. Supports Telegram (typing, upload_photo, etc.) with
// extensibility for future channels.
//
// The action calls Launch's internal /channel-action endpoint which
// handles the actual SDK call. This keeps channel credentials and
// API logic in Launch while allowing flows to trigger actions.
package channel_action

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
	Name         = "Channel Action"
	Description  = "Send a channel-specific action like a typing indicator. Use this before long-running operations to show the user the agent is working."
	Website      = "https://www.flomation.co"
	Icon         = "channel-action"
	Date         = "14/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "channel_type",
		Type:        core.ConnectionTypeString,
		Label:       "Channel Type",
		Placeholder: "telegram, slack, etc.",
		Required:    true,
		Options: []core.ConnectionOption{
			{Name: "Telegram", Value: "telegram"},
			{Name: "Slack", Value: "slack"},
		},
	},
	{
		Name:        "action",
		Type:        core.ConnectionTypeString,
		Label:       "Action",
		Placeholder: "typing",
		Required:    true,
		Options: []core.ConnectionOption{
			{Name: "Typing", Value: "typing"},
			{Name: "Upload Photo", Value: "upload_photo"},
			{Name: "Upload Document", Value: "upload_document"},
			{Name: "Record Video", Value: "record_video"},
			{Name: "Record Voice", Value: "record_voice"},
			{Name: "Find Location", Value: "find_location"},
		},
	},
	{
		Name:        "chat_id",
		Type:        core.ConnectionTypeString,
		Label:       "Chat ID",
		Placeholder: "Target chat/channel ID",
		Required:    true,
	},
	{
		Name:  "agent_id",
		Type:  core.ConnectionTypeString,
		Label: "Agent ID",
	},
}

var Outputs = [...]core.Connection{
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "message", Type: core.ConnectionTypeString, Label: "Result message"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	channelType := requireString("channel_type", inputs)
	action := requireString("action", inputs)
	chatID := requireString("chat_id", inputs)

	if channelType == "" || action == "" || chatID == "" {
		return map[string]interface{}{
			"success": false,
			"message": "channel_type, action, and chat_id are required",
		}, nil
	}

	agentID := optionalString("agent_id", inputs)

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return map[string]interface{}{
			"success": false,
			"message": "execution context with API URL is required",
		}, nil
	}

	// If agent_id not explicitly provided, use context.
	if agentID == "" {
		agentID = ctx.AgentID
	}

	// Call the API's internal channel-action endpoint, which proxies to Launch.
	body, _ := json.Marshal(map[string]string{
		"channel_type": channelType,
		"action":       action,
		"chat_id":      chatID,
	})

	endpoint := fmt.Sprintf("%s/api/v1/internal/agent/%s/channel-action", ctx.APIURL, agentID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("failed to call channel action: %v", err),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	if resp.StatusCode == http.StatusOK {
		return map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("%s action sent to %s", action, channelType),
		}, nil
	}

	return map[string]interface{}{
		"success": false,
		"message": fmt.Sprintf("channel action returned %d: %s", resp.StatusCode, string(respBody)),
	}, nil
}

func requireString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

func optionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}
