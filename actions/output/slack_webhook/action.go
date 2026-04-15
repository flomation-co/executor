package output_slack_webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Slack Webhook"
	Description  = "Send a message to a Slack channel via webhook with mrkdwn formatting and optional Block Kit layouts"
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
		Placeholder: "Hello from Flomation! Use *bold*, _italic_, `code`",
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
	{
		Name:        "blocks",
		Type:        core.ConnectionTypeText,
		Label:       "Block Kit JSON",
		Placeholder: `[{"type":"section","text":{"type":"mrkdwn","text":"*Rich* message"}}]`,
	},
	{
		Name:        "attachments",
		Type:        core.ConnectionTypeText,
		Label:       "Attachments JSON",
		Placeholder: `[{"color":"#36a64f","text":"Attachment text"}]`,
	},
}

var Outputs = [...]core.Connection{
	{Name: "status_code", Type: core.ConnectionTypeInteger},
	{Name: "success", Type: core.ConnectionTypeBoolean},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
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
		"text":   *messageConn.String(),
		"mrkdwn": true,
	}

	if uc := core.FindConnection("username", inputs); uc != nil && uc.String() != nil && *uc.String() != "" {
		payload["username"] = *uc.String()
	}
	if ic := core.FindConnection("icon_emoji", inputs); ic != nil && ic.String() != nil && *ic.String() != "" {
		payload["icon_emoji"] = *ic.String()
	}

	// Block Kit blocks
	if blocksConn := core.FindConnection("blocks", inputs); blocksConn != nil && blocksConn.String() != nil {
		blocksJSON := strings.TrimSpace(*blocksConn.String())
		if blocksJSON != "" {
			var blocks []interface{}
			if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Warn("failed to parse blocks JSON — sending as plain text")
			} else {
				payload["blocks"] = blocks
			}
		}
	}

	// Attachments
	if attachConn := core.FindConnection("attachments", inputs); attachConn != nil && attachConn.String() != nil {
		attachJSON := strings.TrimSpace(*attachConn.String())
		if attachJSON != "" {
			var attachments []interface{}
			if err := json.Unmarshal([]byte(attachJSON), &attachments); err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Warn("failed to parse attachments JSON — skipping")
			} else {
				payload["attachments"] = attachments
			}
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body)) // #nosec G107 — user-provided webhook URL
	if err != nil {
		return map[string]interface{}{
			"status_code": 0,
			"success":     false,
			"error":       err.Error(),
		}, nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"success":     resp.StatusCode == 200,
		"error":       "",
	}, nil
}
