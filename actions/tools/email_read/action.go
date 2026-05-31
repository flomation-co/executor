// Package email_read is a tool action that searches and reads emails
// from connected Google Gmail accounts. Supports searching by query,
// listing recent emails, and reading a specific email in full.
package email_read

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Gmail Read"
	Description  = "Search and read the USER's emails from their connected Gmail. When the user says 'my emails' or 'check my inbox', use this tool — it reads THEIR mailbox. Use Gmail search (e.g. 'is:unread newer_than:1d', 'from:someone') or email_id for a specific message."
	Website      = "https://www.flomation.co"
	Icon         = "gmail"
	Date         = "08/04/2026"
	Type         = core.ActionTypeAction

	gmailAPIBase    = "https://gmail.googleapis.com/gmail/v1/users/me"
	maxBodyBytes    = 10 * 1024 // 10KB truncation for tool_result text
	defaultMaxItems = 10
)

var Inputs = [...]core.Connection{
	{
		Name:        "query",
		Type:        core.ConnectionTypeString,
		Label:       "Gmail search query (e.g. 'from:boss@company.com', 'subject:urgent', 'is:unread')",
		Placeholder: "is:unread",
	},
	{
		Name:        "max_results",
		Type:        core.ConnectionTypeInteger,
		Label:       "Max emails to return (default 10, max 50)",
		Placeholder: "10",
	},
	{
		Name:        "email_id",
		Type:        core.ConnectionTypeString,
		Label:       "Email ID to read in full (from a prior search)",
	},
	{
		Name:        "account",
		Type:        core.ConnectionTypeString,
		Label:       "Filter to a specific account email. Leave empty to search ALL user accounts. Never default to the agent's own email.",
	},
	{
		Name:        "credential",
		Type:        core.ConnectionTypeString,
		Label:       "Google OAuth Credential (optional, overrides user tokens)",
		Placeholder: "${credentials.GOOGLE_EMAIL}",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Results (text)"},
	{Name: "emails", Type: core.ConnectionTypeObject, Label: "Emails (JSON)"},
	{Name: "total_count", Type: core.ConnectionTypeInteger, Label: "Total Results"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type tokenInfo struct {
	Email       string `json:"email"`
	Label       string `json:"label"`
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

type emailSummary struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Snippet string `json:"snippet"`
	Date    string `json:"date"`
	Labels  string `json:"labels"`
	Account string `json:"account"`
}

type emailFull struct {
	emailSummary
	BodyText string `json:"body_text"`
	BodyHTML string `json:"body_html,omitempty"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	query := optionalString("query", inputs)
	emailID := optionalString("email_id", inputs)
	accountFilter := optionalString("account", inputs)
	maxResults := optionalInt("max_results", inputs)
	if maxResults <= 0 {
		maxResults = defaultMaxItems
	}
	if maxResults > 50 {
		maxResults = 50
	}

	credential := optionalString("credential", inputs)

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}
	if credential == "" && ctx.AgentUserID == "" {
		return errResult("No user identity or credential available — provide a credential or connect a user email")
	}

	// Fetch tokens — from credential or connected user accounts
	var tokens []tokenInfo
	if credential != "" {
		tokens = []tokenInfo{{Email: "credential", AccessToken: credential}}
	} else {
		var err error
		tokens, err = fetchTokens(flow, ctx, "email_read")
		if err != nil {
			return errResult(fmt.Sprintf("Failed to get email tokens: %v", err))
		}
	}
	if len(tokens) == 0 {
		return errResult("No Gmail read access connected. Ask the user to connect their email (read access) first.")
	}

	log.WithFields(log.Fields{
		"token_count":    len(tokens),
		"account_filter": accountFilter,
	}).Info("[email_read] tokens fetched")
	for i, t := range tokens {
		log.WithFields(log.Fields{
			"index":     i,
			"email":     t.Email,
			"label":     t.Label,
			"has_token": t.AccessToken != "",
			"error":     t.Error,
		}).Info("[email_read] token detail")
	}

	// Filter accounts
	activeTokens := filterTokens(tokens, accountFilter)
	if len(activeTokens) == 0 {
		return errResult(fmt.Sprintf("No matching email account for filter '%s'", accountFilter))
	}

	log.WithFields(log.Fields{
		"active_count": len(activeTokens),
	}).Info("[email_read] after filter")

	// If a specific email ID is requested, read it in full
	if emailID != "" {
		return readFullEmail(flow, activeTokens, emailID)
	}

	// Otherwise, search/list across all accounts
	return searchEmails(flow, activeTokens, query, maxResults)
}

func searchEmails(flow *core.Flow, tokens []tokenInfo, query string, maxResults int) (map[string]interface{}, error) {
	var allEmails []emailSummary

	// Track emails per account for grouped output
	type accountResult struct {
		Label  string
		Email  string
		Emails []emailSummary
		Error  string
	}
	var accountResults []accountResult

	for _, t := range tokens {
		label := t.Label
		if label == "" {
			label = t.Email
		}
		log.WithFields(log.Fields{
			"account":       t.Email,
			"label":         label,
			"token_prefix":  t.AccessToken[:min(20, len(t.AccessToken))],
		}).Info("[email_read] querying Gmail for account")

		emails, err := listMessages(flow, t.AccessToken, query, maxResults)
		if err != nil {
			log.WithFields(log.Fields{
				"account": t.Email,
				"error":   err,
			}).Warn("[email_read] listMessages failed for account")

			errMsg := err.Error()
			// If permissions expired, auto-disconnect and give a clear message
			if strings.Contains(errMsg, "can't access") {
				disconnectAccount(flow, t.Email, "email_read")
				errMsg = fmt.Sprintf("I can no longer access %s, so I've removed it from your connected accounts. You can re-connect it at any time.", t.Email)
			}

			accountResults = append(accountResults, accountResult{
				Label: label, Email: t.Email, Error: errMsg,
			})
			continue
		}

		log.WithFields(log.Fields{
			"account":     t.Email,
			"email_count": len(emails),
		}).Info("[email_read] results for account")
		if len(emails) > 0 {
			log.WithFields(log.Fields{
				"account": t.Email,
				"subject": emails[0].Subject,
				"from":    emails[0].From,
				"id":      emails[0].ID,
			}).Info("[email_read] latest email for account")
		}

		for i := range emails {
			emails[i].Account = t.Email
		}
		accountResults = append(accountResults, accountResult{
			Label: label, Email: t.Email, Emails: emails,
		})
		allEmails = append(allEmails, emails...)
	}

	if len(allEmails) == 0 {
		// Check whether the empty result is due to errors (403, etc.)
		// vs genuinely empty inboxes — report errors to the AI so it
		// knows the account is inaccessible, not just empty.
		var errMsgs []string
		for _, ar := range accountResults {
			if ar.Error != "" {
				errMsgs = append(errMsgs, fmt.Sprintf("%s (%s): %s", ar.Label, ar.Email, ar.Error))
			}
		}
		if len(errMsgs) > 0 {
			msg := fmt.Sprintf("Couldn't read emails from %d account(s):\n%s",
				len(errMsgs), strings.Join(errMsgs, "\n"))
			return map[string]interface{}{
				"tool_result":  msg,
				"emails":       []emailSummary{},
				"total_count":  0,
				"success":      false,
				"error":        msg,
			}, nil
		}

		searchDesc := "inbox"
		if query != "" {
			searchDesc = fmt.Sprintf("'%s'", query)
		}
		return map[string]interface{}{
			"tool_result":  fmt.Sprintf("No emails found matching %s.", searchDesc),
			"emails":       []emailSummary{},
			"total_count":  0,
			"success":      true,
			"error":        "",
		}, nil
	}

	// Build text summary grouped by account so the AI can clearly
	// distinguish which emails belong to which inbox.
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d email(s) found across %d account(s):\n", len(allEmails), len(accountResults))
	for _, ar := range accountResults {
		fmt.Fprintf(&sb, "\n── %s (%s) ──\n", ar.Label, ar.Email)
		if ar.Error != "" {
			fmt.Fprintf(&sb, "  %s\n", ar.Error)
			continue
		}
		if len(ar.Emails) == 0 {
			sb.WriteString("  No matching emails\n")
			continue
		}
		for _, e := range ar.Emails {
			fmt.Fprintf(&sb, "• From: %s\n  Subject: %s\n  Date: %s\n  Snippet: %s\n  {id:%s}\n\n",
				e.From, e.Subject, e.Date, e.Snippet, e.ID)
		}
	}

	return map[string]interface{}{
		"tool_result":  sb.String(),
		"emails":       allEmails,
		"total_count":  len(allEmails),
		"success":      true,
		"error":        "",
	}, nil
}

func readFullEmail(flow *core.Flow, tokens []tokenInfo, emailID string) (map[string]interface{}, error) {
	// Try each account until we find the email
	for _, t := range tokens {
		email, err := getMessage(flow, t.AccessToken, emailID)
		if err != nil {
			continue
		}
		label := t.Label
		if label == "" {
			label = t.Email
		}
		email.Account = label

		var sb strings.Builder
		fmt.Fprintf(&sb, "From: %s\nTo: %s\nSubject: %s\nDate: %s\nAccount: %s\n\n",
			email.From, email.To, email.Subject, email.Date, email.Account)

		body := email.BodyText
		if body == "" {
			body = stripHTML(email.BodyHTML)
		}
		if len(body) > maxBodyBytes {
			body = body[:maxBodyBytes] + "\n... [truncated]"
		}
		sb.WriteString(body)

		return map[string]interface{}{
			"tool_result":  sb.String(),
			"emails":       []emailFull{*email},
			"total_count":  1,
			"success":      true,
			"error":        "",
		}, nil
	}

	return errResult(fmt.Sprintf("Email %s not found on any connected account", emailID))
}

// --- Gmail API calls ---

func listMessages(flow *core.Flow, accessToken, query string, maxResults int) ([]emailSummary, error) {
	endpoint := fmt.Sprintf("%s/messages?maxResults=%d", gmailAPIBase, maxResults)
	if query != "" {
		endpoint += "&q=" + strings.ReplaceAll(query, " ", "+")
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		log.WithFields(log.Fields{
			"status": resp.StatusCode,
			"body":   string(body),
		}).Warn("[email_read] Gmail API error response")
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("I can't access this inbox — it looks like the email permissions have expired or been revoked. The user will need to re-connect this account")
		}
		return nil, fmt.Errorf("Gmail API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Messages []struct {
			ID       string `json:"id"`
			ThreadID string `json:"threadId"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Fetch metadata for each message
	var emails []emailSummary
	for _, msg := range result.Messages {
		summary, err := getMessageMetadata(flow, accessToken, msg.ID)
		if err != nil {
			continue
		}
		emails = append(emails, *summary)
	}

	return emails, nil
}

func getMessageMetadata(flow *core.Flow, accessToken, messageID string) (*emailSummary, error) {
	endpoint := fmt.Sprintf("%s/messages/%s?format=metadata&metadataHeaders=From&metadataHeaders=To&metadataHeaders=Subject&metadataHeaders=Date",
		gmailAPIBase, messageID)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gmail API returned %d", resp.StatusCode)
	}

	var msg struct {
		ID       string `json:"id"`
		Snippet  string `json:"snippet"`
		LabelIDs []string `json:"labelIds"`
		Payload  struct {
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, err
	}

	summary := &emailSummary{
		ID:      msg.ID,
		Snippet: msg.Snippet,
		Labels:  strings.Join(msg.LabelIDs, ", "),
	}
	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "From":
			summary.From = h.Value
		case "To":
			summary.To = h.Value
		case "Subject":
			summary.Subject = h.Value
		case "Date":
			summary.Date = h.Value
		}
	}

	return summary, nil
}

func getMessage(flow *core.Flow, accessToken, messageID string) (*emailFull, error) {
	endpoint := fmt.Sprintf("%s/messages/%s?format=full", gmailAPIBase, messageID)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gmail API returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var msg struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
		Snippet  string `json:"snippet"`
		LabelIDs []string `json:"labelIds"`
		Payload  struct {
			MimeType string `json:"mimeType"`
			Headers  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
			Body struct {
				Data string `json:"data"`
			} `json:"body"`
			Parts []mimePart `json:"parts"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}

	email := &emailFull{
		emailSummary: emailSummary{
			ID:      msg.ID,
			Snippet: msg.Snippet,
			Labels:  strings.Join(msg.LabelIDs, ", "),
		},
	}

	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "From":
			email.From = h.Value
		case "To":
			email.To = h.Value
		case "Subject":
			email.Subject = h.Value
		case "Date":
			email.Date = h.Value
		}
	}

	// Extract body from MIME structure
	email.BodyText, email.BodyHTML = extractBody(msg.Payload.MimeType, msg.Payload.Body.Data, msg.Payload.Parts)

	return email, nil
}

// --- MIME body extraction ---

type mimePart struct {
	MimeType string `json:"mimeType"`
	Body     struct {
		Data string `json:"data"`
	} `json:"body"`
	Parts []mimePart `json:"parts"`
}

func extractBody(mimeType, bodyData string, parts []mimePart) (plainText, htmlText string) {
	// Simple body (no parts)
	if bodyData != "" {
		decoded := decodeBase64URL(bodyData)
		if strings.HasPrefix(mimeType, "text/plain") {
			return decoded, ""
		}
		if strings.HasPrefix(mimeType, "text/html") {
			return "", decoded
		}
	}

	// Walk parts recursively
	for _, part := range parts {
		pt, ht := extractBody(part.MimeType, part.Body.Data, part.Parts)
		if pt != "" && plainText == "" {
			plainText = pt
		}
		if ht != "" && htmlText == "" {
			htmlText = ht
		}
	}

	return plainText, htmlText
}

func decodeBase64URL(s string) string {
	// Gmail uses base64url encoding (no padding)
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	// Add padding
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s // return raw if decode fails
	}
	return string(decoded)
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

func stripHTML(html string) string {
	text := htmlTagRegex.ReplaceAllString(html, "")
	// Collapse whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// --- Shared helpers ---

func fetchTokens(flow *core.Flow, ctx *core.ExecutionContext, purpose string) ([]tokenInfo, error) {
	var all []tokenInfo
	client := ctx.InternalClient()
	if ctx.AgentUserID != "" {
		if tokens := fetchTokensFrom(flow, client, fmt.Sprintf("%s/api/v1/internal/agent-user/%s/google-tokens?purpose=%s",
			ctx.APIURL, ctx.AgentUserID, purpose)); len(tokens) > 0 {
			all = append(all, tokens...)
		}
	}
	// Fall back to agent-level tokens (configured via Agent → Channels → Email)
	// when no user-specific tokens are available. These are stored in
	// trigger_google_account with the agent ID as the scope key.
	if len(all) == 0 && ctx.AgentID != "" {
		if tokens := fetchTokensFrom(flow, client, fmt.Sprintf("%s/api/v1/internal/trigger/%s/google-tokens?purpose=%s",
			ctx.APIURL, ctx.AgentID, purpose)); len(tokens) > 0 {
			all = append(all, tokens...)
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

func filterTokens(tokens []tokenInfo, accountFilter string) []tokenInfo {
	var active []tokenInfo
	for _, t := range tokens {
		if t.Error != "" {
			continue
		}
		if accountFilter != "" {
			if !strings.EqualFold(t.Email, accountFilter) &&
				!strings.EqualFold(t.Label, accountFilter) &&
				!strings.Contains(strings.ToLower(t.Email), strings.ToLower(accountFilter)) {
				continue
			}
		}
		active = append(active, t)
	}
	return active
}

// disconnectAccount removes a Google account connection that is no longer
// accessible (e.g. 403/401 from Gmail). Fire-and-forget — errors are logged
// but don't block the email read response.
func disconnectAccount(flow *core.Flow, email, purpose string) {
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" || ctx.AgentUserID == "" {
		return
	}
	endpoint := fmt.Sprintf("%s/api/v1/internal/agent-user/%s/google-account/%s?purpose=%s",
		ctx.APIURL, ctx.AgentUserID, email, purpose)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodDelete, endpoint, nil)
	if err != nil {
		log.WithError(err).Warn("[email_read] failed to build disconnect request")
		return
	}
	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		log.WithError(err).Warn("[email_read] failed to disconnect account")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	log.WithFields(log.Fields{
		"email":  email,
		"status": resp.StatusCode,
	}).Info("[email_read] disconnected expired account")
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":  msg,
		"emails":       nil,
		"total_count":  0,
		"success":      false,
		"error":        msg,
	}, nil
}

func optionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func optionalInt(name string, inputs []*core.Connection) int {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return 0
	}
	if n := c.Number(); n != nil {
		return int(*n)
	}
	return 0
}