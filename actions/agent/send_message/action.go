package send_message

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Agent Message"
	Description  = "Send a message through an agent's communication channel and record it"
	Website      = "https://www.flomation.co"
	Icon         = "paper-plane"
	Date         = "03/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "agent_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent ID",
		Placeholder: "ID of the agent",
		Required:    true,
	},
	{
		Name:     "channel",
		Type:     core.ConnectionTypeString,
		Label:    "Channel",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Telegram", Value: "telegram"},
			{Name: "Email", Value: "email"},
			{Name: "Webhook", Value: "webhook"},
		},
	},
	{
		Name:        "recipient",
		Type:        core.ConnectionTypeString,
		Label:       "Recipient",
		Placeholder: "Chat ID, email address, or webhook URL",
		Required:    true,
	},
	{
		Name:        "message",
		Type:        core.ConnectionTypeText,
		Label:       "Message",
		Placeholder: "Message content",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentIDConn := core.FindConnection("agent_id", inputs)
	if agentIDConn == nil || agentIDConn.String() == nil || *agentIDConn.String() == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	agentID := *agentIDConn.String()

	channelConn := core.FindConnection("channel", inputs)
	if channelConn == nil || channelConn.String() == nil || *channelConn.String() == "" {
		return nil, fmt.Errorf("channel is required")
	}
	channel := *channelConn.String()

	recipientConn := core.FindConnection("recipient", inputs)
	if recipientConn == nil || recipientConn.String() == nil || *recipientConn.String() == "" {
		return nil, fmt.Errorf("recipient is required")
	}

	messageConn := core.FindConnection("message", inputs)
	if messageConn == nil || messageConn.String() == nil || *messageConn.String() == "" {
		return nil, fmt.Errorf("message is required")
	}
	message := *messageConn.String()

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	// Record the outbound message via the API
	payload, err := json.Marshal(map[string]interface{}{
		"direction":    "outbound",
		"channel_type": channel,
		"content":      message,
		"sender":       "agent",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/message", ctx.APIURL, agentID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to record message: %w", err)
	}
	defer resp.Body.Close()

	var msgResult map[string]interface{}
	if resp.StatusCode == http.StatusCreated {
		json.NewDecoder(resp.Body).Decode(&msgResult)
	}

	messageID := ""
	if msgResult != nil {
		if id, ok := msgResult["id"].(string); ok {
			messageID = id
		}
	}

	// TODO: Phase 4 — dispatch actual message delivery via channel
	// For now, the message is recorded but not delivered externally.
	// When Telegram/Email integration is complete, this action will
	// call the appropriate channel API to deliver the message.

	return map[string]interface{}{
		"message_id": messageID,
		"success":    resp.StatusCode == http.StatusCreated,
	}, nil
}
