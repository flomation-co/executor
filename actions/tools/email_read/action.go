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
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Email Read"
	Description  = "Search and read emails from connected Gmail accounts. Use a Gmail search query (e.g. 'from:dave@flomation.co', 'subject:invoice', 'is:unread') or provide an email_id to read a specific email in full."
	Website      = "https://www.flomation.co"
	Icon         = "envelope"
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
		Label:       "Account filter (email or label, empty for all)",
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

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}
	if ctx.AgentUserID == "" {
		return errResult("No user identity available — email requires a connected user")
	}

	// Fetch tokens for email_read purpose
	tokens, err := fetchTokens(flow, ctx, "email_read")
	if err != nil {
		return errResult(fmt.Sprintf("Failed to get email tokens: %v", err))
	}
	if len(tokens) == 0 {
		return errResult("No Gmail read access connected. Ask the user to connect their email (read access) first.")
	}

	// Filter accounts
	activeTokens := filterTokens(tokens, accountFilter)
	if len(activeTokens) == 0 {
		return errResult(fmt.Sprintf("No matching email account for filter '%s'", accountFilter))
	}

	// If a specific email ID is requested, read it in full
	if emailID != "" {
		return readFullEmail(flow, activeTokens, emailID)
	}

	// Otherwise, search/list across all accounts
	return searchEmails(flow, activeTokens, query, maxResults)
}

func searchEmails(flow *core.Flow, tokens []tokenInfo, query string, maxResults int) (map[string]interface{}, error) {
	var allEmails []emailSummary

	for _, t := range tokens {
		emails, err := listMessages(flow, t.AccessToken, query, maxResults)
		if err != nil {
			continue
		}
		label := t.Label
		if label == "" {
			label = t.Email
		}
		for i := range emails {
			emails[i].Account = label
		}
		allEmails = append(allEmails, emails...)
	}

	if len(allEmails) == 0 {
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

	// Build text summary
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d email(s) found:\n\n", len(allEmails))
	for _, e := range allEmails {
		fmt.Fprintf(&sb, "• From: %s\n  Subject: %s\n  Date: %s\n  Snippet: %s\n  [%s] {id:%s}\n\n",
			e.From, e.Subject, e.Date, e.Snippet, e.Account, e.ID)
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
		return nil, fmt.Errorf("Gmail API returned %d", resp.StatusCode)
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