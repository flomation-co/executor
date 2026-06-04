// Package calendar_update is a tool action that modifies existing Google
// Calendar events. The AI provides the event_id (from a prior calendar_read)
// and any fields to change.
package calendar_update

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Calendar Update Event"
	Description  = "Update an existing Google Calendar event. Requires the event_id from a previous calendar_read. Only the fields you provide will be changed; others remain untouched."
	Website      = "https://www.flomation.co"
	Icon         = "calendar"
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
		Name:  "title",
		Type:  core.ConnectionTypeString,
		Label: "New title (leave empty to keep current)",
	},
	{
		Name:  "date",
		Type:  core.ConnectionTypeString,
		Label: "New date (YYYY-MM-DD, leave empty to keep current)",
	},
	{
		Name:  "start_time",
		Type:  core.ConnectionTypeString,
		Label: "New start time (HH:MM, leave empty to keep current)",
	},
	{
		Name:  "end_time",
		Type:  core.ConnectionTypeString,
		Label: "New end time (HH:MM, leave empty to keep current)",
	},
	{
		Name:  "description",
		Type:  core.ConnectionTypeString,
		Label: "New description",
	},
	{
		Name:  "location",
		Type:  core.ConnectionTypeString,
		Label: "New location",
	},
	{
		Name:  "account",
		Type:  core.ConnectionTypeString,
		Label: "Account the event is on (email or label)",
	},
	{
		Name:  "scope",
		Type:  core.ConnectionTypeString,
		Label: "For recurring events: 'this_instance' (default), 'all_events' (change the entire series), or 'this_and_following' (this and all future instances)",
		Options: []core.ConnectionOption{
			{Name: "This instance only", Value: "this_instance"},
			{Name: "All events in series", Value: "all_events"},
			{Name: "This and following", Value: "this_and_following"},
		},
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
	scope := optionalString("scope", inputs)
	if scope == "" {
		scope = "this_instance"
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}
	if credential == "" && ctx.AgentUserID == "" {
		return errResult("No user identity or credential available")
	}

	// Fetch tokens — from credential or connected user accounts
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

	// For all_events or this_and_following on a recurring instance,
	// we need to target the master event, not the instance.
	targetID := eventID
	if scope == "all_events" {
		targetID = baseEventID(eventID)
	}

	// Find the event across accounts. If account filter is given, try that
	// first; if it 404s (or no filter given), try all accounts.
	token, existing, err := findEvent(flow, tokens, accountFilter, targetID)
	if err != nil {
		return errResult(err.Error())
	}

	client := &http.Client{Timeout: 15 * time.Second}

	// Apply changes
	changes := 0
	title := optionalString("title", inputs)
	if title != "" {
		existing["summary"] = title
		changes++
	}

	description := optionalString("description", inputs)
	if description != "" {
		existing["description"] = description
		changes++
	}

	location := optionalString("location", inputs)
	if location != "" {
		existing["location"] = location
		changes++
	}

	// Handle date/time changes
	now := time.Now()
	loc := now.Location()
	dateStr := optionalString("date", inputs)
	startTimeStr := optionalString("start_time", inputs)
	endTimeStr := optionalString("end_time", inputs)

	if dateStr != "" || startTimeStr != "" || endTimeStr != "" {
		// Resolve the base date
		var baseDate time.Time
		if dateStr != "" {
			baseDate = resolveDate(dateStr, now)
		} else {
			// Use the existing event's date
			if startObj, ok := existing["start"].(map[string]interface{}); ok {
				if dt, ok := startObj["dateTime"].(string); ok {
					if t, err := time.Parse(time.RFC3339, dt); err == nil {
						baseDate = t
					}
				}
			}
			if baseDate.IsZero() {
				baseDate = now
			}
		}

		if startTimeStr != "" {
			st := parseTime(startTimeStr, baseDate, loc)
			if !st.IsZero() {
				existing["start"] = map[string]string{
					"dateTime": st.Format(time.RFC3339),
					"timeZone": resolveIANATimezone(loc),
				}
				changes++
			}
		} else if dateStr != "" {
			// Date changed but time kept — rebuild with new date and old time
			if startObj, ok := existing["start"].(map[string]interface{}); ok {
				if dt, ok := startObj["dateTime"].(string); ok {
					if t, err := time.Parse(time.RFC3339, dt); err == nil {
						newStart := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(),
							t.Hour(), t.Minute(), 0, 0, loc)
						existing["start"] = map[string]string{
							"dateTime": newStart.Format(time.RFC3339),
							"timeZone": resolveIANATimezone(loc),
						}
						changes++
					}
				}
			}
		}

		if endTimeStr != "" {
			et := parseTime(endTimeStr, baseDate, loc)
			if !et.IsZero() {
				existing["end"] = map[string]string{
					"dateTime": et.Format(time.RFC3339),
					"timeZone": resolveIANATimezone(loc),
				}
				changes++
			}
		} else if dateStr != "" {
			if endObj, ok := existing["end"].(map[string]interface{}); ok {
				if dt, ok := endObj["dateTime"].(string); ok {
					if t, err := time.Parse(time.RFC3339, dt); err == nil {
						newEnd := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(),
							t.Hour(), t.Minute(), 0, 0, loc)
						existing["end"] = map[string]string{
							"dateTime": newEnd.Format(time.RFC3339),
							"timeZone": resolveIANATimezone(loc),
						}
						changes++
					}
				}
			}
		}
	}

	if changes == 0 {
		return errResult("No changes specified — provide at least one field to update")
	}

	// Build the update payload. For recurring series updates we use PATCH
	// with only the changed fields, which preserves recurrence rules.
	// For single instance updates, PUT with the full event body works fine.
	var method string
	var payload []byte
	updateURL := fmt.Sprintf("%s/calendars/primary/events/%s?sendUpdates=all", calendarAPIBase, targetID)

	if scope == "all_events" {
		// PATCH the master event with only changed fields — preserves RRULE
		patch := make(map[string]interface{})
		if title != "" {
			patch["summary"] = title
		}
		if description != "" {
			patch["description"] = description
		}
		if location != "" {
			patch["location"] = location
		}
		if _, ok := existing["start"]; ok && (startTimeStr != "" || dateStr != "") {
			patch["start"] = existing["start"]
		}
		if _, ok := existing["end"]; ok && (endTimeStr != "" || dateStr != "") {
			patch["end"] = existing["end"]
		}
		payload, _ = json.Marshal(patch)
		method = http.MethodPatch
	} else if scope == "this_and_following" {
		// For this_and_following, the instance becomes the new "start" of a
		// modified series. We PUT the instance with the updated fields and
		// the recurrence rule from the master, which splits the series.
		masterID := baseEventID(eventID)
		if masterID != eventID {
			// Fetch the master to get its recurrence rule
			_, master, masterErr := findEvent(flow, tokens, accountFilter, masterID)
			if masterErr == nil {
				if rrule, ok := master["recurrence"]; ok {
					existing["recurrence"] = rrule
				}
			}
		}
		updateURL = fmt.Sprintf("%s/calendars/primary/events/%s?sendUpdates=all", calendarAPIBase, eventID)
		payload, _ = json.Marshal(existing)
		method = http.MethodPut
	} else {
		// this_instance — PUT the full instance body
		payload, _ = json.Marshal(existing)
		method = http.MethodPut
	}

	updateReq, err := http.NewRequestWithContext(flow.GoContext(), method, updateURL, bytes.NewReader(payload))
	if err != nil {
		return errResult(fmt.Sprintf("Failed to build request: %v", err))
	}
	updateReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	updateReq.Header.Set("Content-Type", "application/json")

	updateResp, err := client.Do(updateReq)
	if err != nil {
		return errResult(fmt.Sprintf("Google Calendar API error: %v", err))
	}
	defer func() { _ = updateResp.Body.Close() }()

	if updateResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(updateResp.Body, 512))
		return errResult(fmt.Sprintf("Failed to update (%d): %s", updateResp.StatusCode, string(respBody)))
	}

	summary := "event"
	if s, ok := existing["summary"].(string); ok {
		summary = s
	}

	scopeDesc := ""
	switch scope {
	case "all_events":
		scopeDesc = " (all events in series)"
	case "this_and_following":
		scopeDesc = " (this and following events)"
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated '%s' (%d field(s) changed)%s", summary, changes, scopeDesc),
		"success":     true,
		"error":       "",
	}, nil
}

// --- Shared helpers (duplicated from calendar_create to avoid cross-package deps) ---

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

func resolveDate(s string, now time.Time) time.Time {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "today", "":
		return now
	case "tomorrow":
		return now.AddDate(0, 0, 1)
	}
	dayNames := map[string]time.Weekday{
		"monday": time.Monday, "tuesday": time.Tuesday,
		"wednesday": time.Wednesday, "thursday": time.Thursday,
		"friday": time.Friday, "saturday": time.Saturday,
		"sunday": time.Sunday,
	}
	cleaned := strings.TrimPrefix(strings.TrimPrefix(s, "next "), "this ")
	if target, ok := dayNames[cleaned]; ok {
		daysAhead := int(target) - int(now.Weekday())
		if daysAhead <= 0 {
			daysAhead += 7
		}
		return now.AddDate(0, 0, daysAhead)
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	return now
}

func parseTime(s string, date time.Time, loc *time.Location) time.Time {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("15:04", s); err == nil {
		return time.Date(date.Year(), date.Month(), date.Day(),
			t.Hour(), t.Minute(), 0, 0, loc)
	}
	return time.Time{}
}

func resolveIANATimezone(loc *time.Location) string {
	name := loc.String()
	if name != "Local" && name != "" {
		return name
	}
	if tz := os.Getenv("TZ"); tz != "" && tz != "Local" {
		return tz
	}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if idx := strings.Index(target, "zoneinfo/"); idx != -1 {
			return target[idx+len("zoneinfo/"):]
		}
	}
	_, offset := time.Now().Zone()
	switch offset {
	case 0:
		return "Europe/London"
	case 3600:
		return "Europe/London"
	default:
		return "UTC"
	}
}

// findEvent searches for an event by ID across all connected accounts.
// If accountFilter is given, that account is tried first. Returns the
// matching token and the decoded event body.
func findEvent(flow *core.Flow, tokens []tokenInfo, accountFilter, eventID string) (*tokenInfo, map[string]interface{}, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	// Build ordered list: filtered account first, then the rest
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
		return nil, nil, fmt.Errorf("no valid Google Calendar tokens available")
	}

	getURL := fmt.Sprintf("%s/calendars/primary/events/%s", calendarAPIBase, eventID)
	for i := range ordered {
		req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, getURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+ordered[i].AccessToken)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var existing map[string]interface{}
			err := json.NewDecoder(resp.Body).Decode(&existing)
			_ = resp.Body.Close()
			if err != nil {
				continue
			}
			return &ordered[i], existing, nil
		}
		_ = resp.Body.Close()
	}

	return nil, nil, fmt.Errorf("event %s not found on any connected account", eventID)
}

// baseEventID extracts the master recurring event ID from an instance ID.
// Google Calendar instance IDs have the format "baseId_YYYYMMDDTHHMMSSZ".
// If the ID doesn't contain an underscore, it's already the base ID.
func baseEventID(instanceID string) string {
	if idx := strings.LastIndex(instanceID, "_"); idx > 0 {
		// Verify the suffix looks like a date (YYYYMMDD...)
		suffix := instanceID[idx+1:]
		if len(suffix) >= 8 {
			return instanceID[:idx]
		}
	}
	return instanceID
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
