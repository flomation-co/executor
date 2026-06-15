// Package slack_user_profile is an AI agent tool for fetching a Slack user's full profile.
package slack_user_profile

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Slack User Profile"
	Description  = "Get a Slack user's full profile by user ID. Returns name, email, title, phone, timezone, status, and avatar"
	Website      = "https://www.flomation.co"
	Icon         = "slack+user"
	Date         = "20/04/2026"
	Type         = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{Name: "bot_token", Type: core.ConnectionTypeSecret, Label: "Bot Token", Placeholder: "xoxb-...", Required: true},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Slack user ID (e.g. U01ABCDEF). Use slack_users to find IDs by name", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "profile", Type: core.ConnectionTypeObject, Label: "Full user profile"},
	{Name: "real_name", Type: core.ConnectionTypeString, Label: "Real name"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display name"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email address"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Job title"},
	{Name: "timezone", Type: core.ConnectionTypeString, Label: "Timezone"},
	{Name: "status_text", Type: core.ConnectionTypeString, Label: "Status text"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	botToken := str("bot_token", inputs)
	if botToken == "" {
		return nil, fmt.Errorf("bot_token is required")
	}
	userID := str("user_id", inputs)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	params := url.Values{}
	params.Set("user", userID)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, slackAPIBase+"/users.info?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail("Slack API error: " + err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fail("Failed to parse Slack response")
	}
	if ok, _ := result["ok"].(bool); !ok {
		errMsg, _ := result["error"].(string)
		return fail("Slack API error: " + errMsg)
	}

	user, _ := result["user"].(map[string]interface{})
	profile, _ := user["profile"].(map[string]interface{})

	realName, _ := user["real_name"].(string)
	tz, _ := user["tz"].(string)
	displayName, _ := profile["display_name"].(string)
	email, _ := profile["email"].(string)
	title, _ := profile["title"].(string)
	phone, _ := profile["phone"].(string)
	statusText, _ := profile["status_text"].(string)
	statusEmoji, _ := profile["status_emoji"].(string)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Profile for %s", realName))
	if displayName != "" && displayName != realName {
		sb.WriteString(fmt.Sprintf(" (%s)", displayName))
	}
	sb.WriteString(":\n")
	if email != "" {
		sb.WriteString(fmt.Sprintf("  Email: %s\n", email))
	}
	if title != "" {
		sb.WriteString(fmt.Sprintf("  Title: %s\n", title))
	}
	if phone != "" {
		sb.WriteString(fmt.Sprintf("  Phone: %s\n", phone))
	}
	if tz != "" {
		sb.WriteString(fmt.Sprintf("  Timezone: %s\n", tz))
	}
	if statusText != "" {
		status := statusText
		if statusEmoji != "" {
			status = statusEmoji + " " + status
		}
		sb.WriteString(fmt.Sprintf("  Status: %s\n", status))
	}

	return map[string]interface{}{
		"tool_result":  sb.String(),
		"profile":      profile,
		"real_name":    realName,
		"display_name": displayName,
		"email":        email,
		"title":        title,
		"timezone":     tz,
		"status_text":  statusText,
		"success":      true,
		"error":        "",
	}, nil
}

func fail(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg, "profile": nil, "real_name": "", "display_name": "",
		"email": "", "title": "", "timezone": "", "status_text": "",
		"success": false, "error": msg,
	}, nil
}

func str(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}
