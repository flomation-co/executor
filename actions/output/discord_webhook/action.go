package output_discord_webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Discord Webhook"
	Description  = "Send a message to a Discord channel via webhook"
	Website      = "https://www.flomation.co"
	Icon         = "comment-dots"
	Date         = "23/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "webhook_url",
		Type:        core.ConnectionTypeString,
		Label:       "Webhook URL",
		Placeholder: "https://discord.com/api/webhooks/...",
		Required:    true,
	},
	{
		Name:        "content",
		Type:        core.ConnectionTypeText,
		Label:       "Message Content",
		Placeholder: "Hello from Flomation!",
		Required:    true,
	},
	{
		Name:        "username",
		Type:        core.ConnectionTypeString,
		Label:       "Username Override",
		Placeholder: "Flomation Bot",
	},
	{
		Name:        "avatar_url",
		Type:        core.ConnectionTypeString,
		Label:       "Avatar URL",
		Placeholder: "https://example.com/avatar.png",
	},
}

var Outputs = [...]core.Connection{
	{Name: "status_code", Type: core.ConnectionTypeInteger},
	{Name: "success", Type: core.ConnectionTypeBoolean},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	webhookConn := core.FindConnection("webhook_url", inputs)
	if webhookConn == nil || webhookConn.String() == nil || *webhookConn.String() == "" {
		return nil, fmt.Errorf("webhook_url is required")
	}
	webhookURL := *webhookConn.String()

	if !strings.HasPrefix(webhookURL, "https://") {
		return nil, fmt.Errorf("webhook URL must use HTTPS")
	}

	contentConn := core.FindConnection("content", inputs)
	if contentConn == nil || contentConn.String() == nil || *contentConn.String() == "" {
		return nil, fmt.Errorf("content is required")
	}

	payload := map[string]interface{}{
		"content": *contentConn.String(),
	}

	if uc := core.FindConnection("username", inputs); uc != nil && uc.String() != nil && *uc.String() != "" {
		payload["username"] = *uc.String()
	}
	if ac := core.FindConnection("avatar_url", inputs); ac != nil && ac.String() != nil && *ac.String() != "" {
		payload["avatar_url"] = *ac.String()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"success":     resp.StatusCode >= 200 && resp.StatusCode < 300,
	}, nil
}
