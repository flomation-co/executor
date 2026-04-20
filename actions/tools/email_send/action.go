// Package email_send is a tool action that sends emails via connected
// Gmail accounts. Composes a MIME message and sends via the Gmail API.
package email_send

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Email Send"
	Description  = "Send an email. Default sends from agent's account. Set account to user's email to send on their behalf."
	Website      = "https://www.flomation.co"
	Icon         = "envelope"
	Date         = "08/04/2026"
	Type         = core.ActionTypeAction

	gmailAPIBase = "https://gmail.googleapis.com/gmail/v1/users/me"
)

var Inputs = [...]core.Connection{
	{
		Name:     "to",
		Type:     core.ConnectionTypeString,
		Label:    "Recipient email address(es), comma-separated",
		Required: true,
	},
	{
		Name:     "subject",
		Type:     core.ConnectionTypeString,
		Label:    "Email subject",
		Required: true,
	},
	{
		Name:     "body",
		Type:     core.ConnectionTypeText,
		Label:    "Email body / message content (plain text)",
		Required: true,
	},
	{
		Name:  "cc",
		Type:  core.ConnectionTypeString,
		Label: "CC recipients (comma-separated)",
	},
	{
		Name:  "bcc",
		Type:  core.ConnectionTypeString,
		Label: "BCC recipients (comma-separated)",
	},
	{
		Name:  "sender_name",
		Type:  core.ConnectionTypeString,
		Label: "Sender display name (e.g. 'Ada Whitmore', defaults to account label)",
	},
	{
		Name:  "account",
		Type:  core.ConnectionTypeString,
		Label: "Sending account (email or label, empty for primary)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result (text)"},
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "thread_id", Type: core.ConnectionTypeString, Label: "Thread ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type tokenInfo struct {
	Email       string `json:"email"`
	Label       string `json:"label"`
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	to := requireString("to", inputs)
	subject := requireString("subject", inputs)
	body := requireString("body", inputs)
	// Accept "message" as an alias for "body" — AI models
	// frequently use "message" as the natural parameter name.
	if body == "" {
		body = requireString("message", inputs)
	}
	if to == "" || subject == "" || body == "" {
		return errResult("to, subject, and body are all required")
	}

	cc := optionalString("cc", inputs)
	bcc := optionalString("bcc", inputs)
	accountFilter := optionalString("account", inputs)

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}
	if ctx.AgentUserID == "" {
		return errResult("No user identity available — email requires a connected user")
	}

	tokens, err := fetchTokens(flow, ctx, "email_send")
	if err != nil {
		return errResult(fmt.Sprintf("Failed to get email tokens: %v", err))
	}
	if len(tokens) == 0 {
		return errResult("No Gmail send access connected. Ask the user to connect their email (send access) first.")
	}

	token, err := pickAccount(tokens, accountFilter)
	if err != nil {
		return errResult(err.Error())
	}

	// Build RFC 2822 MIME message
	senderName := optionalString("sender_name", inputs)

	var msg strings.Builder
	if senderName != "" {
		msg.WriteString(fmt.Sprintf("From: \"%s\" <%s>\r\n", senderName, token.Email))
	} else {
		msg.WriteString(fmt.Sprintf("From: %s\r\n", token.Email))
	}
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	if cc != "" {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
	}
	if bcc != "" {
		msg.WriteString(fmt.Sprintf("Bcc: %s\r\n", bcc))
	}
	// Replace common non-ASCII characters that cause encoding issues
	// in email subjects. The AI often uses em/en dashes and smart quotes.
	subjectClean := strings.NewReplacer(
		"\u2014", "-", // em dash → hyphen
		"\u2013", "-", // en dash → hyphen
		"\u2018", "'", // left single quote
		"\u2019", "'", // right single quote
		"\u201c", "\"", // left double quote
		"\u201d", "\"", // right double quote
		"\u2026", "...", // ellipsis
	).Replace(subject)
	// RFC 2047 encode subject for any remaining non-ASCII.
	encodedSubject := mime.QEncoding.Encode("UTF-8", subjectClean)
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", encodedSubject))
	msg.WriteString("MIME-Version: 1.0\r\n")

	// Detect HTML content and set the appropriate Content-Type.
	// If the body looks like HTML, send as text/html so email clients
	// render it properly instead of showing raw tags.
	isHTML := strings.Contains(body, "<html") || strings.Contains(body, "<HTML") ||
		strings.Contains(body, "<body") || strings.Contains(body, "<div") ||
		strings.Contains(body, "<p>") || strings.Contains(body, "<br") ||
		strings.Contains(body, "<table") || strings.Contains(body, "<h1") ||
		strings.Contains(body, "<h2") || strings.Contains(body, "<h3")
	if isHTML {
		msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	} else {
		msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	// Base64url encode the message
	raw := base64.URLEncoding.EncodeToString([]byte(msg.String()))

	// Send via Gmail API
	payload, _ := json.Marshal(map[string]string{"raw": raw})
	endpoint := fmt.Sprintf("%s/messages/send", gmailAPIBase)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return errResult(fmt.Sprintf("Failed to build request: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("Gmail API error: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Sprintf("Gmail API returned %d: %s", resp.StatusCode, string(respBody)))
	}

	var result struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return errResult(fmt.Sprintf("Failed to parse response: %v", err))
	}

	recipients := to
	if cc != "" {
		recipients += " (cc: " + cc + ")"
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Email sent to %s from %s. Subject: %s", recipients, token.Email, subject),
		"message_id":  result.ID,
		"thread_id":   result.ThreadID,
		"success":     true,
		"error":       "",
	}, nil
}

// --- Shared helpers ---

func fetchTokens(flow *core.Flow, ctx *core.ExecutionContext, purpose string) ([]tokenInfo, error) {
	var all []tokenInfo
	client := &http.Client{Timeout: 15 * time.Second}
	if ctx.AgentUserID != "" {
		if tokens := fetchTokensFrom(flow, client, fmt.Sprintf("%s/api/v1/internal/agent-user/%s/google-tokens?purpose=%s",
			ctx.APIURL, ctx.AgentUserID, purpose)); len(tokens) > 0 {
			all = append(all, tokens...)
		}
	}
	if ctx.AgentID != "" {
		if tokens := fetchTokensFrom(flow, client, fmt.Sprintf("%s/api/v1/internal/trigger/%s/google-tokens?purpose=%s",
			ctx.APIURL, ctx.AgentID, purpose)); len(tokens) > 0 {
			seen := make(map[string]bool)
			for _, t := range all {
				seen[t.Email] = true
			}
			for _, t := range tokens {
				if !seen[t.Email] {
					all = append(all, t)
				}
			}
		}
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no %s tokens available", purpose)
	}
	return all, nil
}

func fetchTokensFrom(flow *core.Flow, client *http.Client, endpoint string) []tokenInfo {
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var tokens []tokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil
	}
	return tokens
}

func pickAccount(tokens []tokenInfo, filter string) (*tokenInfo, error) {
	var valid []tokenInfo
	for _, t := range tokens {
		if t.Error == "" {
			valid = append(valid, t)
		}
	}
	if len(valid) == 0 {
		return nil, fmt.Errorf("no valid Gmail send tokens available")
	}
	if filter == "" {
		return &valid[0], nil
	}
	for i, t := range valid {
		if strings.EqualFold(t.Email, filter) ||
			strings.EqualFold(t.Label, filter) ||
			strings.Contains(strings.ToLower(t.Email), strings.ToLower(filter)) {
			return &valid[i], nil
		}
	}
	var names []string
	for _, t := range valid {
		names = append(names, t.Email)
	}
	return nil, fmt.Errorf("no matching account for '%s'. Available: %s", filter, strings.Join(names, ", "))
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"message_id":  "",
		"thread_id":   "",
		"success":     false,
		"error":       msg,
	}, nil
}

func requireString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func optionalString(name string, inputs []*core.Connection) string {
	return requireString(name, inputs)
}