// Package calendar_create is a tool action that creates events on Google
// Calendar. The AI specifies the account to use (email or label); if
// ambiguous, the tool defaults to the primary account.
package calendar_create

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
	Name         = "Calendar Create Event"
	Description  = "Create a new event on a connected Google Calendar. Specify the account (email or label like 'Work'/'Personal'), date, time, title, and optional attendees/location."
	Website      = "https://www.flomation.co"
	Icon         = "calendar"
	Date         = "08/04/2026"
	Type         = core.ActionTypeAction

	calendarAPIBase = "https://www.googleapis.com/calendar/v3"
)

var Inputs = [...]core.Connection{
	{
		Name:        "title",
		Type:        core.ConnectionTypeString,
		Label:       "Event title",
		Required:    true,
	},
	{
		Name:        "date",
		Type:        core.ConnectionTypeString,
		Label:       "Date (YYYY-MM-DD, 'today', 'tomorrow', or day name)",
		Required:    true,
	},
	{
		Name:        "start_time",
		Type:        core.ConnectionTypeString,
		Label:       "Start time (HH:MM, 24h format)",
		Required:    true,
	},
	{
		Name:        "end_time",
		Type:        core.ConnectionTypeString,
		Label:       "End time (HH:MM, 24h format)",
		Required:    true,
	},
	{
		Name:        "description",
		Type:        core.ConnectionTypeString,
		Label:       "Event description or notes",
	},
	{
		Name:        "attendees",
		Type:        core.ConnectionTypeString,
		Label:       "Attendees (comma-separated email addresses)",
	},
	{
		Name:        "location",
		Type:        core.ConnectionTypeString,
		Label:       "Event location",
	},
	{
		Name:        "account",
		Type:        core.ConnectionTypeString,
		Label:       "Account (email, label like 'Work'/'Personal', or empty for primary)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result (text)"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Created Event ID"},
	{Name: "event_link", Type: core.ConnectionTypeString, Label: "Event Link"},
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
	title := requireString("title", inputs)
	if title == "" {
		return errResult("Event title is required")
	}
	dateStr := requireString("date", inputs)
	startTime := requireString("start_time", inputs)
	endTime := requireString("end_time", inputs)
	if dateStr == "" || startTime == "" || endTime == "" {
		return errResult("Date, start_time, and end_time are all required")
	}

	description := optionalString("description", inputs)
	attendeesStr := optionalString("attendees", inputs)
	location := optionalString("location", inputs)
	accountFilter := optionalString("account", inputs)

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}
	if ctx.AgentUserID == "" {
		return errResult("No user identity available — calendar requires a connected user")
	}

	// Resolve the date
	now := time.Now()
	loc := now.Location()
	eventDate := resolveDate(dateStr, now)

	// Parse start and end times
	startParsed := parseTime(startTime, eventDate, loc)
	endParsed := parseTime(endTime, eventDate, loc)
	if startParsed.IsZero() || endParsed.IsZero() {
		return errResult(fmt.Sprintf("Could not parse times: start=%q end=%q", startTime, endTime))
	}

	// Fetch tokens
	tokens, err := fetchTokens(flow, ctx)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to get calendar tokens: %v", err))
	}
	if len(tokens) == 0 {
		return errResult("No Google Calendar accounts connected. Ask the user to connect their calendar first.")
	}

	// Pick the target account
	token, err := pickAccount(tokens, accountFilter)
	if err != nil {
		return errResult(err.Error())
	}

	// Build the Google Calendar event. RFC3339 includes the UTC offset
	// so Google can determine the timezone from the dateTime alone —
	// no separate timeZone field needed (avoids "Local" timezone issue).
	tzName := resolveIANATimezone(loc)
	event := map[string]interface{}{
		"summary": title,
		"start": map[string]string{
			"dateTime": startParsed.Format(time.RFC3339),
			"timeZone": tzName,
		},
		"end": map[string]string{
			"dateTime": endParsed.Format(time.RFC3339),
			"timeZone": tzName,
		},
	}
	if description != "" {
		event["description"] = description
	}
	if location != "" {
		event["location"] = location
	}
	if attendeesStr != "" {
		var attendees []map[string]string
		for _, email := range strings.Split(attendeesStr, ",") {
			email = strings.TrimSpace(email)
			if email != "" {
				attendees = append(attendees, map[string]string{"email": email})
			}
		}
		if len(attendees) > 0 {
			event["attendees"] = attendees
		}
	}

	// POST to Google Calendar API
	// sendUpdates=all ensures attendees receive email invitations.
	body, _ := json.Marshal(event)
	endpoint := fmt.Sprintf("%s/calendars/primary/events?sendUpdates=all", calendarAPIBase)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return errResult(fmt.Sprintf("Failed to build request: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("Google Calendar API error: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return errResult(fmt.Sprintf("Google Calendar API returned %d: %s", resp.StatusCode, string(respBody)))
	}

	var result struct {
		ID      string `json:"id"`
		HTMLLink string `json:"htmlLink"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return errResult(fmt.Sprintf("Failed to parse response: %v", err))
	}

	accountLabel := token.Label
	if accountLabel == "" {
		accountLabel = token.Email
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created '%s' on %s at %s–%s on account %s [%s]. Event ID: %s",
			title, eventDate.Format("Monday 2 January"), startTime, endTime, token.Email, accountLabel, result.ID),
		"event_id":   result.ID,
		"event_link": result.HTMLLink,
		"account":    token.Email,
		"success":    true,
		"error":      "",
	}, nil
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
		if t.Error != "" {
			continue
		}
		valid = append(valid, t)
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

	return nil, fmt.Errorf("no matching account for '%s'. Available: %s",
		filter, formatAccountList(valid))
}

func formatAccountList(tokens []tokenInfo) string {
	var parts []string
	for _, t := range tokens {
		label := t.Label
		if label == "" {
			label = "Unlabelled"
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", t.Email, label))
	}
	return strings.Join(parts, ", ")
}

func resolveDate(s string, now time.Time) time.Time {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "today", "":
		return now
	case "tomorrow":
		return now.AddDate(0, 0, 1)
	case "yesterday":
		return now.AddDate(0, 0, -1)
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
	// Try TZ environment variable
	if tz := os.Getenv("TZ"); tz != "" && tz != "Local" {
		return tz
	}
	// On macOS/Linux, read the /etc/localtime symlink target
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		// target is like /var/db/timezone/zoneinfo/Europe/London
		if idx := strings.Index(target, "zoneinfo/"); idx != -1 {
			return target[idx+len("zoneinfo/"):]
		}
	}
	// Fallback: use UTC offset to pick a reasonable IANA name
	_, offset := time.Now().Zone()
	switch offset {
	case 0:
		return "Europe/London"
	case 3600:
		return "Europe/London" // BST
	case 7200:
		return "Europe/Paris"
	default:
		return "UTC"
	}
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"event_id":    "",
		"event_link":  "",
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
