package output_slack_webhook

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
	Name         = "Slack Webhook"
	Description  = "Send a message to a Slack channel via webhook"
	Website      = "https://www.flomation.co"
	Icon         = "hashtag"
	Date         = "23/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "webhook_url",
		Type:        core.ConnectionTypeString,
		Label:       "Webhook URL",
		Placeholder: "https://hooks.slack.com/services/...",
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
		Name:        "username",
		Type:        core.ConnectionTypeString,
		Label:       "Username Override",
		Placeholder: "Flomation Bot",
	},
	{
		Name:        "icon_emoji",
		Type:        core.ConnectionTypeString,
		Label:       "Icon Emoji",
		Placeholder: ":robot_face:",
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

	messageConn := core.FindConnection("message", inputs)
	if messageConn == nil || messageConn.String() == nil || *messageConn.String() == "" {
		return nil, fmt.Errorf("message is required")
	}

	payload := map[string]interface{}{
		"text": *messageConn.String(),
	}

	if uc := core.FindConnection("username", inputs); uc != nil && uc.String() != nil && *uc.String() != "" {
		payload["username"] = *uc.String()
	}
	if ic := core.FindConnection("icon_emoji", inputs); ic != nil && ic.String() != nil && *ic.String() != "" {
		payload["icon_emoji"] = *ic.String()
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
		"success":     resp.StatusCode == 200,
	}, nil
}
