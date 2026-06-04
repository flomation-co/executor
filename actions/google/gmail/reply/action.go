// Package email_reply is a tool action that replies to an existing email
// thread via Gmail. Preserves threading by setting In-Reply-To, References,
// and threadId on the sent message.
package email_reply

import (
	"encoding/base64"
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
	Name         = "Gmail Reply"
	Description  = "Reply to an existing email. Requires the email_id from a previous email_read. Preserves the thread so the reply appears in the correct conversation."
	Website      = "https://www.flomation.co"
	Icon         = "gmail+reply"
	Date         = "08/04/2026"
	Type         = core.ActionTypeAction

	gmailAPIBase = "https://gmail.googleapis.com/gmail/v1/users/me"
)

var Inputs = [...]core.Connection{
	{
		Name:     "email_id",
		Type:     core.ConnectionTypeString,
		Label:    "Email ID to reply to (from email_read)",
		Required: true,
	},
	{
		Name:     "body",
		Type:     core.ConnectionTypeText,
		Label:    "Reply body (plain text)",
		Required: true,
	},
	{
		Name:  "reply_all",
		Type:  core.ConnectionTypeBoolean,
		Label: "Reply all (default false)",
	},
	{
		Name:  "sender_name",
		Type:  core.ConnectionTypeString,
		Label: "Sender display name (e.g. 'Ada Whitmore')",
	},
	{
		Name:  "account",
		Type:  core.ConnectionTypeString,
		Label: "Account to send from (email or label)",
	},
	{
		Name:        "credential",
		Type:        core.ConnectionTypeString,
		Label:       "Google OAuth Credential (optional, overrides user tokens)",
		Placeholder: "${credentials.GOOGLE_EMAIL}",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result (text)"},
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
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
	emailID := requireString("email_id", inputs)
	body := requireString("body", inputs)
	if emailID == "" {
		return errResult("email_id is required — use email_read first to get the email_id")
	}
	if body == "" {
		return errResult("body is required — provide the reply text")
	}

	accountFilter := optionalString("account", inputs)
	replyAll := false
	if c := core.FindConnection("reply_all", inputs); c != nil {
		if v, ok := c.Value.(bool); ok {
			replyAll = v
		} else if s := c.String(); s != nil && strings.EqualFold(*s, "true") {
			replyAll = true
		}
	}

	credential := optionalString("credential", inputs)

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}
	if credential == "" && ctx.AgentUserID == "" {
		return errResult("No user identity or credential available")
	}

	// Need both send tokens (to send) and read tokens (to fetch original)
	var sendTokens []tokenInfo
	if credential != "" {
		sendTokens = []tokenInfo{{Email: "credential", AccessToken: credential}}
	} else {
		var err error
		sendTokens, err = fetchTokens(flow, ctx, "email_send")
		if err != nil {
			return errResult(fmt.Sprintf("Failed to get send tokens: %v", err))
		}
	}
	if len(sendTokens) == 0 {
		return errResult("No Gmail send access connected.")
	}

	sendToken, err := pickAccount(sendTokens, accountFilter)
	if err != nil {
		return errResult(err.Error())
	}

	// Fetch the original message to get threading headers.
	// Try read tokens first, fall back to send tokens (which may have read access).
	readTokens, _ := fetchTokens(flow, ctx, "email_read")
	allTokens := append(readTokens, sendTokens...)

	original, err := fetchOriginal(flow, allTokens, emailID)
	if err != nil {
		return errResult(fmt.Sprintf("Could not fetch original email: %v", err))
	}

	// Build the reply — always reply to the original sender.
	// For reply-all, CC everyone else (original To + CC) minus ourselves.
	replyTo := original.from
	var ccLine string
	if replyAll {
		var ccAddrs []string
		// Gather all original recipients
		for _, addr := range splitAddresses(original.to) {
			ccAddrs = append(ccAddrs, addr)
		}
		for _, addr := range splitAddresses(original.cc) {
			ccAddrs = append(ccAddrs, addr)
		}
		// Remove ourselves and the sender (already in To) from CC
		var filtered []string
		senderLower := strings.ToLower(sendToken.Email)
		replyToLower := strings.ToLower(extractEmail(original.from))
		for _, addr := range ccAddrs {
			lower := strings.ToLower(extractEmail(addr))
			if lower != senderLower && lower != replyToLower && lower != "" {
				filtered = append(filtered, addr)
			}
		}
		if len(filtered) > 0 {
			ccLine = strings.Join(filtered, ", ")
		}
	}

	subject := original.subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	senderName := optionalString("sender_name", inputs)
	var msg strings.Builder
	if senderName != "" {
		msg.WriteString(fmt.Sprintf("From: \"%s\" <%s>\r\n", senderName, sendToken.Email))
	} else {
		msg.WriteString(fmt.Sprintf("From: %s\r\n", sendToken.Email))
	}
	msg.WriteString(fmt.Sprintf("To: %s\r\n", replyTo))
	if ccLine != "" {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", ccLine))
	}
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	if original.messageID != "" {
		msg.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", original.messageID))
		msg.WriteString(fmt.Sprintf("References: %s\r\n", original.messageID))
	}
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	raw := base64.URLEncoding.EncodeToString([]byte(msg.String()))

	payload, _ := json.Marshal(map[string]interface{}{
		"raw":      raw,
		"threadId": original.threadID,
	})

	endpoint := fmt.Sprintf("%s/messages/send", gmailAPIBase)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return errResult(fmt.Sprintf("Failed to build request: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+sendToken.AccessToken)
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
		ID string `json:"id"`
	}
	json.Unmarshal(respBody, &result)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Reply sent to %s from %s. Subject: %s", replyTo, sendToken.Email, subject),
		"message_id":  result.ID,
		"success":     true,
		"error":       "",
	}, nil
}

// --- Original message fetch ---

type originalEmail struct {
	threadID  string
	messageID string // Message-ID header for threading
	from      string
	to        string
	cc        string
	subject   string
}

func fetchOriginal(flow *core.Flow, tokens []tokenInfo, emailID string) (*originalEmail, error) {
	for _, t := range tokens {
		if t.Error != "" || t.AccessToken == "" {
			continue
		}
		endpoint := fmt.Sprintf("%s/messages/%s?format=metadata&metadataHeaders=From&metadataHeaders=To&metadataHeaders=Cc&metadataHeaders=Subject&metadataHeaders=Message-ID",
			gmailAPIBase, emailID)
		req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+t.AccessToken)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var msg struct {
			ThreadID string `json:"threadId"`
			Payload  struct {
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
			} `json:"payload"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
			continue
		}

		orig := &originalEmail{threadID: msg.ThreadID}
		for _, h := range msg.Payload.Headers {
			switch h.Name {
			case "From":
				orig.from = h.Value
			case "To":
				orig.to = h.Value
			case "Cc":
				orig.cc = h.Value
			case "Subject":
				orig.subject = h.Value
			case "Message-ID":
				orig.messageID = h.Value
			}
		}
		return orig, nil
	}
	return nil, fmt.Errorf("email %s not found on any connected account", emailID)
}

// --- Shared helpers ---

func fetchTokens(flow *core.Flow, ctx *core.ExecutionContext, purpose string) ([]tokenInfo, error) {
	var all []tokenInfo
	client := ctx.InternalClient()

	// Source 1: agent-user scoped tokens (per-user Google accounts)
	if ctx.AgentUserID != "" {
		if tokens := fetchTokensFrom(flow, client, fmt.Sprintf("%s/api/v1/internal/agent-user/%s/google-tokens?purpose=%s",
			ctx.APIURL, ctx.AgentUserID, purpose)); len(tokens) > 0 {
			all = append(all, tokens...)
		}
	}

	// Source 2: agent-scoped tokens (configured on the agent's email channel)
	if ctx.AgentID != "" {
		if tokens := fetchTokensFrom(flow, client, fmt.Sprintf("%s/api/v1/internal/trigger/%s/google-tokens?purpose=%s",
			ctx.APIURL, ctx.AgentID, purpose)); len(tokens) > 0 {
			// Dedup by email
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

// splitAddresses splits a comma-separated list of email addresses.
func splitAddresses(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, addr := range strings.Split(s, ",") {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			result = append(result, addr)
		}
	}
	return result
}

// extractEmail pulls the bare email from a "Name <email>" format.
func extractEmail(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "<"); idx != -1 {
		end := strings.Index(s, ">")
		if end > idx {
			return strings.TrimSpace(s[idx+1 : end])
		}
	}
	return s
}
