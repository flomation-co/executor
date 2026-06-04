// Package calendar_delete is a tool action that removes events from
// Google Calendar. The AI provides the event_id from a prior calendar_read.
package calendar_delete

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
	Name         = "Calendar Delete Event"
	Description  = "Delete an event from Google Calendar. Requires the event_id from a previous calendar_read. This permanently removes the event."
	Website      = "https://www.flomation.co"
	Icon         = "calendar+trash"
	Date         = "08/04/2026"
	Type         = core.ActionTypeAction

	calendarAPIBase = "https://www.googleapis.com/calendar/v3"
)

var Inputs = [...]core.Connection{
	{
		Name:     "event_id",
		Type:     core.ConnectionTypeString,
		Label:    "Event ID (from calendar_read results)",
		Required: true,
	},
	{
		Name:  "account",
		Type:  core.ConnectionTypeString,
		Label: "Account the event is on (email or label)",
	},
	{
		Name:        "credential",
		Type:        core.ConnectionTypeString,
		Label:       "Google OAuth Credential (optional, overrides user tokens)",
		Placeholder: "${credentials.GOOGLE_CALENDAR}",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result (text)"},
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
	eventID := optionalString("event_id", inputs)
	if eventID == "" {
		return errResult("event_id is required — use calendar_read first to get event IDs")
	}

	accountFilter := optionalString("account", inputs)
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
		tokens, err = fetchTokens(flow, ctx)
		if err != nil {
			return errResult(fmt.Sprintf("Failed to get calendar tokens: %v", err))
		}
	}

	// Try deleting from the filtered account first, then all others.
	// Event IDs are unique per-account, so we need to find which account owns it.
	client := &http.Client{Timeout: 15 * time.Second}
	deleteURL := fmt.Sprintf("%s/calendars/primary/events/%s", calendarAPIBase, eventID)

	// Build ordered list: filtered account first, then rest
	var ordered []tokenInfo
	var rest []tokenInfo
	for _, t := range tokens {
		if t.Error != "" {
			continue
		}
		if accountFilter != "" && (strings.EqualFold(t.Email, accountFilter) ||
			strings.EqualFold(t.Label, accountFilter) ||
			strings.Contains(strings.ToLower(t.Email), strings.ToLower(accountFilter))) {
			ordered = append([]tokenInfo{t}, ordered...)
		} else {
			rest = append(rest, t)
		}
	}
	ordered = append(ordered, rest...)

	if len(ordered) == 0 {
		return errResult("No valid Google Calendar tokens available")
	}

	for _, t := range ordered {
		req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodDelete, deleteURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+t.AccessToken)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusGone {
			_ = resp.Body.Close()
			return map[string]interface{}{
				"tool_result": fmt.Sprintf("Event deleted successfully from %s (ID: %s)", t.Email, eventID),
				"success":     true,
				"error":       "",
			}, nil
		}
		_ = resp.Body.Close()

		// 404 = not on this account, try next
		if resp.StatusCode == http.StatusNotFound {
			continue
		}

		// Other error = real failure
		return errResult(fmt.Sprintf("Failed to delete event (%d)", resp.StatusCode))
	}

	return errResult(fmt.Sprintf("Event %s not found on any connected account", eventID))
}

// --- Shared helpers ---

func fetchTokens(flow *core.Flow, ctx *core.ExecutionContext) ([]tokenInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/internal/agent-user/%s/google-tokens",
		ctx.APIURL, ctx.AgentUserID)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var tokens []tokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}

func pickAccount(tokens []tokenInfo, filter string) (*tokenInfo, error) {
	var valid []tokenInfo
	for _, t := range tokens {
		if t.Error == "" {
			valid = append(valid, t)
		}
	}
	if len(valid) == 0 {
		return nil, fmt.Errorf("no valid Google Calendar tokens available")
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
