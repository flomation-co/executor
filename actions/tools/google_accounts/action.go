// Package google_accounts is a tool action that manages all Google
// account connections — calendar, email read, and email send. Shows
// per-purpose connection status and provides OAuth links for each purpose.
package google_accounts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Manage Google Connections"
	Description  = "Manage the user's Google account connections for calendar, email read, and email send. Lists which accounts are connected and for which purposes (calendar, email read, email send), provides OAuth links to connect new permissions, and can disconnect specific accounts/purposes. Use this when the user wants to connect or manage their Google accounts."
	Website      = "https://www.flomation.co"
	Icon         = "link"
	Date         = "08/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:     "action",
		Type:     core.ConnectionTypeString,
		Label:    "What to do with Google account connections",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "List accounts with purposes and get connect links", Value: "list"},
			{Name: "Disconnect an account or specific purpose", Value: "disconnect"},
		},
	},
	{
		Name:        "email",
		Type:        core.ConnectionTypeString,
		Label:       "Email (for disconnect)",
		Placeholder: "user@example.com",
	},
	{
		Name:  "purpose",
		Type:  core.ConnectionTypeString,
		Label: "Purpose to disconnect (calendar, email_read, email_send) — empty removes all",
		Options: []core.ConnectionOption{
			{Name: "Calendar access", Value: "calendar"},
			{Name: "Email read access", Value: "email_read"},
			{Name: "Email send access", Value: "email_send"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Results (text)"},
	{Name: "accounts", Type: core.ConnectionTypeObject, Label: "Connected Accounts (JSON)"},
	{Name: "auth_urls", Type: core.ConnectionTypeObject, Label: "OAuth Connect URLs by purpose"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type accountInfo struct {
	Email   string `json:"email"`
	Label   string `json:"label,omitempty"`
	Purpose string `json:"purpose"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	action := optionalString("action", inputs)
	if action == "" {
		action = "list"
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}
	if ctx.AgentUserID == "" {
		return errResult("No user identity available")
	}

	switch action {
	case "disconnect":
		return handleDisconnect(flow, ctx, inputs)
	default:
		return handleList(flow, ctx)
	}
}

func handleList(flow *core.Flow, ctx *core.ExecutionContext) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/api/v1/internal/agent-user/%s/google-accounts?agent_id=%s",
		ctx.APIURL, ctx.AgentUserID, ctx.AgentID)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to build request: %v", err))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to fetch accounts: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return errResult(fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)))
	}

	var result struct {
		Accounts []accountInfo     `json:"accounts"`
		AuthURLs map[string]string `json:"auth_urls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return errResult(fmt.Sprintf("Failed to decode response: %v", err))
	}

	// Group accounts by email, showing which purposes each has
	type emailGroup struct {
		Label    string
		Purposes []string
	}
	grouped := make(map[string]*emailGroup)
	for _, acct := range result.Accounts {
		g, ok := grouped[acct.Email]
		if !ok {
			g = &emailGroup{Label: acct.Label}
			grouped[acct.Email] = g
		}
		g.Purposes = append(g.Purposes, acct.Purpose)
	}

	purposeLabels := map[string]string{
		"calendar":   "calendar",
		"email_read": "email read",
		"email_send": "email send",
	}

	var sb strings.Builder
	if len(grouped) == 0 {
		sb.WriteString("No Google accounts connected.\n")
	} else {
		sb.WriteString("Connected Google accounts:\n")
		for email, g := range grouped {
			label := g.Label
			if label == "" {
				label = "Unlabelled"
			}
			var purposes []string
			for _, p := range g.Purposes {
				if l, ok := purposeLabels[p]; ok {
					purposes = append(purposes, l+" ✓")
				}
			}
			fmt.Fprintf(&sb, "• %s (%s): %s\n", email, label, strings.Join(purposes, ", "))
		}
	}

	if len(result.AuthURLs) > 0 {
		sb.WriteString("\nConnect links:\n")
		urlLabels := map[string]string{
			"calendar":   "Calendar access",
			"email_read": "Email read access",
			"email_send": "Email send access",
		}
		for purpose, url := range result.AuthURLs {
			label := urlLabels[purpose]
			if label == "" {
				label = purpose
			}
			fmt.Fprintf(&sb, "• %s: %s\n", label, url)
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"accounts":    result.Accounts,
		"auth_urls":   result.AuthURLs,
		"success":     true,
		"error":       "",
	}, nil
}

func handleDisconnect(flow *core.Flow, ctx *core.ExecutionContext, inputs []*core.Connection) (map[string]interface{}, error) {
	email := optionalString("email", inputs)
	if email == "" {
		return errResult("Email address is required to disconnect an account")
	}

	purpose := optionalString("purpose", inputs)

	endpoint := fmt.Sprintf("%s/api/v1/internal/agent-user/%s/google-account/%s",
		ctx.APIURL, ctx.AgentUserID, email)
	if purpose != "" {
		endpoint += "?purpose=" + purpose
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodDelete, endpoint, nil)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to build request: %v", err))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to disconnect: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		desc := email
		if purpose != "" {
			desc = fmt.Sprintf("%s (%s)", email, purpose)
		}
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Disconnected %s from Google.", desc),
			"accounts":    nil,
			"auth_urls":   nil,
			"success":     true,
			"error":       "",
		}, nil
	}

	return errResult(fmt.Sprintf("Failed to disconnect %s (status %d)", email, resp.StatusCode))
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"accounts":    nil,
		"auth_urls":   nil,
		"success":     false,
		"error":       msg,
	}, nil
}

func optionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
