// Package email_draft is a tool action that manages Gmail drafts —
// create, list, update, and delete.
package email_draft

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
	Name         = "Gmail Draft"
	Description  = "Manage Gmail drafts: create a new draft, list existing drafts, update a draft, or delete one. Use action='create' to compose without sending, 'list' to see drafts, 'update' to modify, 'delete' to remove."
	Website      = "https://www.flomation.co"
	Icon         = "gmail+file-pen"
	Date         = "08/04/2026"
	Type         = core.ActionTypeAction

	gmailAPIBase = "https://gmail.googleapis.com/gmail/v1/users/me"
)

var Inputs = [...]core.Connection{
	{
		Name:     "action",
		Type:     core.ConnectionTypeString,
		Label:    "Action to perform",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "Create a new draft", Value: "create"},
			{Name: "List existing drafts", Value: "list"},
			{Name: "Update a draft", Value: "update"},
			{Name: "Delete a draft", Value: "delete"},
		},
	},
	{Name: "to", Type: core.ConnectionTypeString, Label: "Recipient (for create/update)"},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject (for create/update)"},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Body (for create/update)"},
	{Name: "draft_id", Type: core.ConnectionTypeString, Label: "Draft ID (for update/delete)"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (email or label)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential (optional)", Placeholder: "${credentials.GOOGLE_EMAIL}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result (text)"},
	{Name: "draft_id", Type: core.ConnectionTypeString, Label: "Draft ID"},
	{Name: "drafts", Type: core.ConnectionTypeObject, Label: "Drafts (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type tokenInfo struct {
	Email       string `json:"email"`
	Label       string `json:"label"`
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

type draftSummary struct {
	DraftID string `json:"draft_id"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Snippet string `json:"snippet"`
	Account string `json:"account"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	action := optionalString("action", inputs)
	if action == "" {
		action = "list"
	}

	credential := optionalString("credential", inputs)

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}
	if credential == "" && ctx.AgentUserID == "" {
		return errResult("No user identity or credential available")
	}

	var tokens []tokenInfo
	if credential != "" {
		tokens = []tokenInfo{{Email: "credential", AccessToken: credential}}
	} else {
		var err error
		tokens, err = fetchTokens(flow, ctx, "email_send")
		if err != nil {
			return errResult(fmt.Sprintf("Failed to get tokens: %v", err))
		}
	}
	if len(tokens) == 0 {
		return errResult("No Gmail send access connected.")
	}

	accountFilter := optionalString("account", inputs)
	token, err := pickAccount(tokens, accountFilter)
	if err != nil {
		return errResult(err.Error())
	}

	switch action {
	case "create":
		return createDraft(flow, token, inputs)
	case "list":
		return listDrafts(flow, tokens, accountFilter)
	case "update":
		return updateDraft(flow, token, inputs)
	case "delete":
		return deleteDraft(flow, token, inputs)
	default:
		return errResult(fmt.Sprintf("Unknown action: %s", action))
	}
}

func createDraft(flow *core.Flow, token *tokenInfo, inputs []*core.Connection) (map[string]interface{}, error) {
	to := optionalString("to", inputs)
	subject := optionalString("subject", inputs)
	body := optionalString("body", inputs)

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", token.Email))
	if to != "" {
		msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	}
	if subject != "" {
		msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	}
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	if body != "" {
		msg.WriteString(body)
	}

	raw := base64.URLEncoding.EncodeToString([]byte(msg.String()))
	payload, _ := json.Marshal(map[string]interface{}{
		"message": map[string]string{"raw": raw},
	})

	endpoint := fmt.Sprintf("%s/drafts", gmailAPIBase)
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
		ID string `json:"id"`
	}
	json.Unmarshal(respBody, &result)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Draft created on %s. To: %s, Subject: %s {draft_id:%s}", token.Email, to, subject, result.ID),
		"draft_id":    result.ID,
		"drafts":      nil,
		"success":     true,
		"error":       "",
	}, nil
}

func listDrafts(flow *core.Flow, tokens []tokenInfo, accountFilter string) (map[string]interface{}, error) {
	var allDrafts []draftSummary

	for _, t := range tokens {
		if t.Error != "" {
			continue
		}
		if accountFilter != "" &&
			!strings.EqualFold(t.Email, accountFilter) &&
			!strings.EqualFold(t.Label, accountFilter) {
			continue
		}

		endpoint := fmt.Sprintf("%s/drafts?maxResults=20", gmailAPIBase)
		req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+t.AccessToken)

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		var result struct {
			Drafts []struct {
				ID      string `json:"id"`
				Message struct {
					ID      string `json:"id"`
					Snippet string `json:"snippet"`
				} `json:"message"`
			} `json:"drafts"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		_ = resp.Body.Close()

		label := t.Label
		if label == "" {
			label = t.Email
		}

		for _, d := range result.Drafts {
			allDrafts = append(allDrafts, draftSummary{
				DraftID: d.ID,
				Snippet: d.Message.Snippet,
				Account: label,
			})
		}
	}

	if len(allDrafts) == 0 {
		return map[string]interface{}{
			"tool_result": "No drafts found.",
			"draft_id":    "",
			"drafts":      []draftSummary{},
			"success":     true,
			"error":       "",
		}, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d draft(s):\n", len(allDrafts))
	for _, d := range allDrafts {
		fmt.Fprintf(&sb, "• %s [%s] {draft_id:%s}\n", d.Snippet, d.Account, d.DraftID)
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"draft_id":    "",
		"drafts":      allDrafts,
		"success":     true,
		"error":       "",
	}, nil
}

func updateDraft(flow *core.Flow, token *tokenInfo, inputs []*core.Connection) (map[string]interface{}, error) {
	draftID := optionalString("draft_id", inputs)
	if draftID == "" {
		return errResult("draft_id is required for update")
	}

	to := optionalString("to", inputs)
	subject := optionalString("subject", inputs)
	body := optionalString("body", inputs)

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", token.Email))
	if to != "" {
		msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	}
	if subject != "" {
		msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	}
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	if body != "" {
		msg.WriteString(body)
	}

	raw := base64.URLEncoding.EncodeToString([]byte(msg.String()))
	payload, _ := json.Marshal(map[string]interface{}{
		"message": map[string]string{"raw": raw},
	})

	endpoint := fmt.Sprintf("%s/drafts/%s", gmailAPIBase, draftID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPut, endpoint, strings.NewReader(string(payload)))
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

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return errResult(fmt.Sprintf("Gmail API returned %d: %s", resp.StatusCode, string(respBody)))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Draft %s updated.", draftID),
		"draft_id":    draftID,
		"drafts":      nil,
		"success":     true,
		"error":       "",
	}, nil
}

func deleteDraft(flow *core.Flow, token *tokenInfo, inputs []*core.Connection) (map[string]interface{}, error) {
	draftID := optionalString("draft_id", inputs)
	if draftID == "" {
		return errResult("draft_id is required for delete")
	}

	endpoint := fmt.Sprintf("%s/drafts/%s", gmailAPIBase, draftID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodDelete, endpoint, nil)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to build request: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("Gmail API error: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Draft %s deleted.", draftID),
			"draft_id":    "",
			"drafts":      nil,
			"success":     true,
			"error":       "",
		}, nil
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return errResult(fmt.Sprintf("Failed to delete draft (%d): %s", resp.StatusCode, string(respBody)))
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
	var errored []tokenInfo
	for _, t := range tokens {
		if t.Error != "" {
			errored = append(errored, t)
			continue
		}
		valid = append(valid, t)
	}
	if len(valid) == 0 {
		if msg := formatTokenErrors(errored); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("no valid Gmail tokens available")
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
	return nil, fmt.Errorf("no matching account for '%s'", filter)
}

// formatTokenErrors renders refresh failures into a single user-facing
// message — see google/calendar/read for the full rationale.
func formatTokenErrors(errored []tokenInfo) string {
	if len(errored) == 0 {
		return ""
	}
	parts := make([]string, 0, len(errored))
	for _, t := range errored {
		label := t.Email
		if label == "" {
			label = t.Label
		}
		if label == "" {
			label = "Google account"
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", label, t.Error))
	}
	return "Google account refresh failed — please re-link the affected Gmail account(s): " + strings.Join(parts, "; ")
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"draft_id":    "",
		"drafts":      nil,
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
