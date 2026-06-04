// Package calendar_read is a tool action that reads events from all
// connected Google Calendar accounts for the current agent_user.
// Supports three query types: events listing, availability check,
// and free slot finding — consolidated into one tool so the AI
// picks the right mode from natural language.
package calendar_read

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Calendar Read"
	Description  = "Read events, check availability, or find free slots across all connected Google calendars"
	Website      = "https://www.flomation.co"
	Icon         = "calendar+eye"
	Date         = "08/04/2026"
	Type         = core.ActionTypeAction

	calendarAPIBase = "https://www.googleapis.com/calendar/v3"
)

var Inputs = [...]core.Connection{
	{
		Name:     "query_type",
		Type:     core.ConnectionTypeString,
		Label:    "Query Type",
		Required: true,
		Options: []core.ConnectionOption{
			{Name: "List events", Value: "events"},
			{Name: "Check availability", Value: "availability"},
			{Name: "Find free slots", Value: "free_slots"},
		},
	},
	{
		Name:        "date_from",
		Type:        core.ConnectionTypeString,
		Label:       "Start Date (YYYY-MM-DD or 'today', 'tomorrow')",
		Placeholder: "today",
		Required:    true,
	},
	{
		Name:        "date_to",
		Type:        core.ConnectionTypeString,
		Label:       "End Date (YYYY-MM-DD, defaults to date_from)",
		Placeholder: "",
	},
	{
		Name:        "start_time",
		Type:        core.ConnectionTypeString,
		Label:       "Start Time (HH:MM, for availability check)",
		Placeholder: "14:00",
	},
	{
		Name:        "end_time",
		Type:        core.ConnectionTypeString,
		Label:       "End Time (HH:MM, for availability check)",
		Placeholder: "15:00",
	},
	{
		Name:        "duration_minutes",
		Type:        core.ConnectionTypeInteger,
		Label:       "Slot Duration (minutes, for free slot search)",
		Placeholder: "60",
	},
	{
		Name:        "account",
		Type:        core.ConnectionTypeString,
		Label:       "Account filter (email, label like 'Work', or empty for all)",
		Placeholder: "",
	},
	{
		Name:        "credential",
		Type:        core.ConnectionTypeString,
		Label:       "Google OAuth Credential (optional, overrides user tokens)",
		Placeholder: "${credentials.GOOGLE_CALENDAR}",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Results (text)"},
	{Name: "events", Type: core.ConnectionTypeObject, Label: "Events (JSON)"},
	{Name: "is_free", Type: core.ConnectionTypeBoolean, Label: "Is Free (availability)"},
	{Name: "free_slots", Type: core.ConnectionTypeObject, Label: "Free Slots (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type calendarEvent struct {
	Title     string `json:"title"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Location  string `json:"location,omitempty"`
	Account   string `json:"account"`
	AllDay    bool   `json:"all_day,omitempty"`
	EventID   string `json:"event_id"`
	Recurring bool   `json:"recurring,omitempty"`
}

type tokenInfo struct {
	Email       string `json:"email"`
	Label       string `json:"label"`
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	queryType := optionalString("query_type", inputs)
	if queryType == "" {
		queryType = "events"
	}

	dateFrom := optionalString("date_from", inputs)
	dateTo := optionalString("date_to", inputs)
	startTime := optionalString("start_time", inputs)
	endTime := optionalString("end_time", inputs)
	durationMin := optionalInt("duration_minutes", inputs)
	accountFilter := optionalString("account", inputs)

	credential := optionalString("credential", inputs)

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}
	if credential == "" && ctx.AgentUserID == "" {
		return errResult("No user identity or credential available — provide a credential or connect a user calendar")
	}

	// Resolve date strings
	now := time.Now()
	loc := now.Location()
	fromDate := resolveDate(dateFrom, now)
	// Only resolve date_to if it was actually provided — empty means
	// "same as date_from", not "today".
	var toDate time.Time
	if strings.TrimSpace(dateTo) != "" {
		toDate = resolveDate(dateTo, now)
	}
	if toDate.IsZero() || toDate.Before(fromDate) {
		toDate = fromDate
	}

	// Build time range
	timeMin := time.Date(fromDate.Year(), fromDate.Month(), fromDate.Day(), 0, 0, 0, 0, loc)
	timeMax := time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 23, 59, 59, 0, loc)

	// For availability check, narrow the window
	if queryType == "availability" && startTime != "" && endTime != "" {
		if st := parseTime(startTime, fromDate, loc); !st.IsZero() {
			timeMin = st
		}
		if et := parseTime(endTime, fromDate, loc); !et.IsZero() {
			timeMax = et
		}
	}

	// Fetch access tokens — from credential or connected user accounts
	var tokens []tokenInfo
	if credential != "" {
		tokens = []tokenInfo{{Email: "credential", AccessToken: credential}}
	} else {
		var err error
		tokens, err = fetchTokens(flow, ctx)
		if err != nil {
			return errResult(fmt.Sprintf("Failed to get calendar tokens: %v", err))
		}
		if len(tokens) == 0 {
			return errResult("No Google Calendar accounts connected. Ask the user to connect their calendar first.")
		}
	}

	// Filter accounts if requested
	var activeTokens []tokenInfo
	for _, t := range tokens {
		if t.Error != "" {
			continue
		}
		if accountFilter != "" {
			matchesEmail := strings.EqualFold(t.Email, accountFilter)
			matchesLabel := strings.EqualFold(t.Label, accountFilter)
			matchesDomain := strings.Contains(strings.ToLower(t.Email), strings.ToLower(accountFilter))
			if !matchesEmail && !matchesLabel && !matchesDomain {
				continue
			}
		}
		activeTokens = append(activeTokens, t)
	}

	if len(activeTokens) == 0 {
		return errResult(fmt.Sprintf("No matching calendar account for filter '%s'", accountFilter))
	}

	// Fetch events from all accounts
	var allEvents []calendarEvent
	for _, t := range activeTokens {
		events, err := fetchEvents(flow, t.AccessToken, timeMin, timeMax)
		if err != nil {
			continue // skip failed accounts, use others
		}
		label := t.Label
		if label == "" {
			label = t.Email
		}
		for i := range events {
			events[i].Account = label
		}
		allEvents = append(allEvents, events...)
	}

	// Sort chronologically
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Start < allEvents[j].Start
	})

	switch queryType {
	case "events":
		return eventsResult(allEvents, fromDate, toDate)
	case "availability":
		return availabilityResult(allEvents, timeMin, timeMax)
	case "free_slots":
		if durationMin <= 0 {
			durationMin = 60
		}
		return freeSlotsResult(allEvents, timeMin, timeMax, durationMin)
	default:
		return eventsResult(allEvents, fromDate, toDate)
	}
}

// --- Google Calendar API ---

func fetchTokens(flow *core.Flow, ctx *core.ExecutionContext) ([]tokenInfo, error) {
	// Get Google client credentials from Launch config via the API.
	// The token endpoint needs client_id and client_secret to refresh.
	// For now, we pass them as query params (they're platform credentials,
	// not user secrets). A future improvement would have the API store
	// these centrally.
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

func fetchEvents(flow *core.Flow, accessToken string, timeMin, timeMax time.Time) ([]calendarEvent, error) {
	params := url.Values{
		"timeMin":      {timeMin.Format(time.RFC3339)},
		"timeMax":      {timeMax.Format(time.RFC3339)},
		"singleEvents": {"true"},
		"orderBy":      {"startTime"},
		"maxResults":   {"50"},
	}

	endpoint := fmt.Sprintf("%s/calendars/primary/events?%s", calendarAPIBase, params.Encode())
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
		return nil, fmt.Errorf("Google Calendar API returned %d", resp.StatusCode)
	}

	var result struct {
		Items []struct {
			ID               string `json:"id"`
			Summary          string `json:"summary"`
			Location         string `json:"location"`
			RecurringEventID string `json:"recurringEventId"`
			Start            struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"end"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var events []calendarEvent
	for _, item := range result.Items {
		ev := calendarEvent{
			Title:     item.Summary,
			Location:  item.Location,
			EventID:   item.ID,
			Recurring: item.RecurringEventID != "",
		}
		if item.Start.DateTime != "" {
			ev.Start = item.Start.DateTime
			ev.End = item.End.DateTime
		} else {
			ev.Start = item.Start.Date
			ev.End = item.End.Date
			ev.AllDay = true
		}
		events = append(events, ev)
	}
	return events, nil
}

// --- Result formatters ---

func eventsResult(events []calendarEvent, from, to time.Time) (map[string]interface{}, error) {
	if len(events) == 0 {
		dateStr := from.Format("Monday, 2 January")
		if from != to {
			dateStr = fmt.Sprintf("%s to %s", from.Format("Monday 2 Jan"), to.Format("Monday 2 Jan"))
		}
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("No events found for %s. Calendar is clear.", dateStr),
			"events":      []calendarEvent{},
			"success":     true,
			"error":       "",
		}, nil
	}

	multiDay := !sameDay(from, to)

	var sb strings.Builder
	var lastDate string
	for _, ev := range events {
		// When spanning multiple days, group by date with headers
		if multiDay {
			evDate := extractDate(ev.Start)
			if evDate != lastDate {
				if lastDate != "" {
					sb.WriteString("\n")
				}
				fmt.Fprintf(&sb, "── %s ──\n", evDate)
				lastDate = evDate
			}
		}

		startStr := formatEventTime(ev.Start)
		endStr := formatEventTime(ev.End)
		if ev.AllDay {
			fmt.Fprintf(&sb, "• All day: %s", ev.Title)
		} else if multiDay {
			// Include full date+time so the AI can't misattribute dates
			startFull := formatEventDateTime(ev.Start)
			endFull := formatEventTime(ev.End)
			fmt.Fprintf(&sb, "• %s–%s: %s", startFull, endFull, ev.Title)
		} else {
			fmt.Fprintf(&sb, "• %s–%s: %s", startStr, endStr, ev.Title)
		}
		if ev.Location != "" {
			fmt.Fprintf(&sb, " (%s)", ev.Location)
		}
		if ev.Recurring {
			sb.WriteString(" [recurring]")
		}
		if ev.Account != "" {
			fmt.Fprintf(&sb, " [%s]", ev.Account)
		}
		// Always include the event ID so the AI can use it for
		// update/delete operations without a second lookup.
		if ev.EventID != "" {
			fmt.Fprintf(&sb, " {id:%s}", ev.EventID)
		}
		sb.WriteString("\n")
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"events":      events,
		"success":     true,
		"error":       "",
	}, nil
}

func availabilityResult(events []calendarEvent, slotStart, slotEnd time.Time) (map[string]interface{}, error) {
	conflicts := filterEventsInRange(events, slotStart, slotEnd)
	isFree := len(conflicts) == 0

	var sb strings.Builder
	if isFree {
		fmt.Fprintf(&sb, "Free from %s to %s — no conflicts across any calendar.",
			slotStart.Format("15:04"), slotEnd.Format("15:04"))
	} else {
		fmt.Fprintf(&sb, "NOT free from %s to %s. Conflicts:\n",
			slotStart.Format("15:04"), slotEnd.Format("15:04"))
		for _, ev := range conflicts {
			fmt.Fprintf(&sb, "• %s–%s: %s [%s]\n",
				formatEventTime(ev.Start), formatEventTime(ev.End), ev.Title, ev.Account)
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"events":      conflicts,
		"is_free":     isFree,
		"success":     true,
		"error":       "",
	}, nil
}

func freeSlotsResult(events []calendarEvent, dayStart, dayEnd time.Time, minMinutes int) (map[string]interface{}, error) {
	minDuration := time.Duration(minMinutes) * time.Minute

	// Build busy periods from events
	type period struct{ start, end time.Time }
	var busy []period
	for _, ev := range events {
		if ev.AllDay {
			continue
		}
		s, _ := time.Parse(time.RFC3339, ev.Start)
		e, _ := time.Parse(time.RFC3339, ev.End)
		if !s.IsZero() && !e.IsZero() {
			busy = append(busy, period{s, e})
		}
	}

	// Merge overlapping periods
	sort.Slice(busy, func(i, j int) bool { return busy[i].start.Before(busy[j].start) })
	var merged []period
	for _, b := range busy {
		if len(merged) > 0 && b.start.Before(merged[len(merged)-1].end) {
			if b.end.After(merged[len(merged)-1].end) {
				merged[len(merged)-1].end = b.end
			}
		} else {
			merged = append(merged, b)
		}
	}

	// Find gaps
	type slot struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	var freeSlots []slot
	cursor := dayStart
	for _, b := range merged {
		if b.start.Sub(cursor) >= minDuration {
			freeSlots = append(freeSlots, slot{
				Start: cursor.Format("15:04"),
				End:   b.start.Format("15:04"),
			})
		}
		if b.end.After(cursor) {
			cursor = b.end
		}
	}
	if dayEnd.Sub(cursor) >= minDuration {
		freeSlots = append(freeSlots, slot{
			Start: cursor.Format("15:04"),
			End:   dayEnd.Format("15:04"),
		})
	}

	var sb strings.Builder
	if len(freeSlots) == 0 {
		sb.WriteString(fmt.Sprintf("No free slots of %d+ minutes found.", minMinutes))
	} else {
		sb.WriteString(fmt.Sprintf("Free slots (%d+ minutes):\n", minMinutes))
		for _, s := range freeSlots {
			fmt.Fprintf(&sb, "• %s–%s\n", s.Start, s.End)
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"free_slots":  freeSlots,
		"success":     true,
		"error":       "",
	}, nil
}

// --- Helpers ---

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"events":      nil,
		"is_free":     false,
		"free_slots":  nil,
		"success":     false,
		"error":       msg,
	}, nil
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

	// Relative day offsets: "+1", "+3", "+7"
	if strings.HasPrefix(s, "+") {
		if days, err := strconv.Atoi(s[1:]); err == nil {
			return now.AddDate(0, 0, days)
		}
	}

	// Named day targets: "monday", "tuesday", etc.
	dayNames := map[string]time.Weekday{
		"monday": time.Monday, "tuesday": time.Tuesday,
		"wednesday": time.Wednesday, "thursday": time.Thursday,
		"friday": time.Friday, "saturday": time.Saturday,
		"sunday": time.Sunday,
	}
	// Also handle "next monday", "this friday"
	cleaned := strings.TrimPrefix(strings.TrimPrefix(s, "next "), "this ")
	if target, ok := dayNames[cleaned]; ok {
		daysAhead := int(target) - int(now.Weekday())
		if daysAhead <= 0 {
			daysAhead += 7
		}
		// "this friday" when today is wednesday = 2 days ahead (not 9)
		if strings.HasPrefix(s, "this ") && daysAhead > 7 {
			daysAhead -= 7
		}
		return now.AddDate(0, 0, daysAhead)
	}

	// End-of-week shorthand
	switch s {
	case "end of week", "end of this week", "eow":
		daysToSunday := int(time.Sunday+7-now.Weekday()) % 7
		if daysToSunday == 0 {
			daysToSunday = 7
		}
		return now.AddDate(0, 0, daysToSunday)
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

func formatEventTime(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("15:04")
	}
	return s
}

func formatEventDateTime(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("Mon 2 Jan 15:04")
	}
	return s
}

func extractDate(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("Monday 2 January")
	}
	// All-day events use YYYY-MM-DD format
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("Monday 2 January")
	}
	return s
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func filterEventsInRange(events []calendarEvent, rangeStart, rangeEnd time.Time) []calendarEvent {
	var result []calendarEvent
	for _, ev := range events {
		if ev.AllDay {
			result = append(result, ev)
			continue
		}
		evStart, _ := time.Parse(time.RFC3339, ev.Start)
		evEnd, _ := time.Parse(time.RFC3339, ev.End)
		if evStart.IsZero() || evEnd.IsZero() {
			continue
		}
		// Overlaps if event starts before range ends AND event ends after range starts
		if evStart.Before(rangeEnd) && evEnd.After(rangeStart) {
			result = append(result, ev)
		}
	}
	return result
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