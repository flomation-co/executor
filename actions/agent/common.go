// Shared helpers for the agent action category. This package holds no
// Execute function, so the manifest generator skips it.
//
// The date resolver lives here because two callers need identical
// behaviour: process_extraction, which turns an extracted promise into a
// commitment after the fact, and set_reminder, which an agent calls
// during its own turn. Both must resolve timing on the platform rather
// than in the model — every commitment a model dated itself on live was
// wrong, and every one this resolver dated was right.
package agent_actions

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var months = map[string]time.Month{
	"january": time.January, "jan": time.January,
	"february": time.February, "feb": time.February,
	"march": time.March, "mar": time.March,
	"april": time.April, "apr": time.April,
	"may":  time.May,
	"june": time.June, "jun": time.June,
	"july": time.July, "jul": time.July,
	"august": time.August, "aug": time.August,
	"september": time.September, "sep": time.September, "sept": time.September,
	"october": time.October, "oct": time.October,
	"november": time.November, "nov": time.November,
	"december": time.December, "dec": time.December,
}

var weekdays = map[string]time.Weekday{
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
	"sunday": time.Sunday, "sun": time.Sunday,
}

// DefaultReminderHour is when a reminder lands if the request names a
// day but no time. Morning, because "remind me on the 1st" means during
// that day rather than as it begins.
const DefaultReminderHour = 9

// ResolveWhen turns a human time expression into an absolute instant.
//
// It accepts durations ("in 72 hours", "30m"), relative days
// ("tomorrow at 9am", "next Monday"), and calendar dates ("1 October",
// "October 1st at 14:30", "2026-10-01"). A date with no year resolves
// to its next occurrence, so it can never land in the past.
//
// Returns ok=false for anything it cannot read, which the caller should
// treat as unschedulable rather than guess at.
func ResolveWhen(expression string, now time.Time) (time.Time, bool) {
	s := strings.ToLower(strings.TrimSpace(expression))
	s = strings.TrimSuffix(s, ".")
	if s == "" {
		return time.Time{}, false
	}
	s = strings.TrimPrefix(s, "on ")

	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return now.Add(d), true
	}
	if when, ok := resolveNamedRelative(s, now); ok {
		return when, ok
	}
	if when, ok := resolveCalendarDate(s, now); ok {
		return when, ok
	}
	return resolveCountedDuration(s, now)
}

// ResolveWhenRFC3339 is ResolveWhen in the wire format the commitment
// API expects. Returns "" when the expression cannot be read.
func ResolveWhenRFC3339(expression string, now time.Time) string {
	when, ok := ResolveWhen(expression, now)
	if !ok {
		return ""
	}
	return when.UTC().Format(time.RFC3339)
}

func resolveNamedRelative(s string, now time.Time) (time.Time, bool) {
	if strings.HasPrefix(s, "in a ") || strings.HasPrefix(s, "in an ") {
		rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "in an "), "in a "))
		switch {
		case strings.HasPrefix(rest, "hour"):
			return now.Add(time.Hour), true
		case strings.HasPrefix(rest, "minute"):
			return now.Add(time.Minute), true
		case strings.HasPrefix(rest, "day"):
			return now.AddDate(0, 0, 1), true
		case strings.HasPrefix(rest, "week"):
			return now.AddDate(0, 0, 7), true
		case strings.HasPrefix(rest, "fortnight"):
			return now.AddDate(0, 0, 14), true
		case strings.HasPrefix(rest, "month"):
			return now.AddDate(0, 1, 0), true
		case strings.HasPrefix(rest, "year"):
			return now.AddDate(1, 0, 0), true
		}
	}

	for prefix, days := range map[string]int{"today": 0, "tonight": 0, "tomorrow": 1} {
		if !strings.HasPrefix(s, prefix) {
			continue
		}
		day := now.AddDate(0, 0, days)
		if at, ok := timeOfDay(s, day); ok {
			return at, true
		}
		if prefix == "tonight" {
			return dayAt(day, 19, 0), true
		}
		if prefix == "today" {
			return dayAt(day, DefaultReminderHour, 0), true
		}
		return time.Date(day.Year(), day.Month(), day.Day(), now.Hour(), now.Minute(), 0, 0, day.Location()), true
	}

	rest := strings.TrimPrefix(s, "next ")
	if weekday, ok := weekdays[firstWord(rest)]; ok {
		day := nextWeekday(weekday, now)
		if at, ok := timeOfDay(rest, day); ok {
			return at, true
		}
		return dayAt(day, DefaultReminderHour, 0), true
	}

	return time.Time{}, false
}

// resolveCalendarDate reads "1 october", "october 1st", "1/10/2026",
// "2026-10-01" and "the 14th", each optionally followed by a time. A
// missing year is filled with the next occurrence rather than the
// current one, so a date can never resolve into the past.
func resolveCalendarDate(s string, now time.Time) (time.Time, bool) {
	datePart, timePart := splitAt(s)

	if iso, err := time.Parse("2006-01-02", datePart); err == nil {
		day := time.Date(iso.Year(), iso.Month(), iso.Day(), 0, 0, 0, 0, now.Location())
		return withTime(day, timePart, now), true
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04"} {
		if exact, err := time.Parse(layout, strings.ToUpper(s)); err == nil {
			return exact, true
		}
	}

	fields := strings.Fields(strings.ReplaceAll(datePart, ",", " "))
	if len(fields) == 0 {
		return time.Time{}, false
	}

	if len(fields) >= 2 && fields[0] == "the" {
		fields = fields[1:]
	}

	// "1 october [2027]" or "october 1st [2027]"
	var day int
	var month time.Month
	var year int
	found := 0
	for _, field := range fields {
		if m, ok := months[strings.TrimSuffix(field, ".")]; ok {
			month = m
			found |= 1
			continue
		}
		if n, ok := ordinal(field); ok {
			if n > 31 && n < 3000 {
				year = n
				continue
			}
			day = n
			found |= 2
		}
	}
	if found != 3 {
		// A bare day of month — "the 14th" — means the next 14th.
		if found == 2 && day > 0 && len(fields) == 1 {
			return nextDayOfMonth(day, timePart, now), true
		}
		return time.Time{}, false
	}

	if year == 0 {
		year = now.Year()
		candidate := time.Date(year, month, day, 23, 59, 59, 0, now.Location())
		if candidate.Before(now) {
			year++
		}
	}
	return withTime(time.Date(year, month, day, 0, 0, 0, 0, now.Location()), timePart, now), true
}

func resolveCountedDuration(s string, now time.Time) (time.Time, bool) {
	fields := strings.Fields(strings.TrimPrefix(s, "in "))
	if len(fields) < 2 {
		return time.Time{}, false
	}
	var n float64
	if _, err := fmt.Sscanf(fields[0], "%f", &n); err != nil || n <= 0 {
		return time.Time{}, false
	}
	switch strings.TrimSuffix(fields[1], "s") {
	case "second":
		return now.Add(time.Duration(n) * time.Second), true
	case "minute":
		return now.Add(time.Duration(n) * time.Minute), true
	case "hour":
		return now.Add(time.Duration(n) * time.Hour), true
	case "day":
		return now.Add(time.Duration(n) * 24 * time.Hour), true
	case "week":
		return now.Add(time.Duration(n) * 7 * 24 * time.Hour), true
	case "fortnight":
		return now.Add(time.Duration(n) * 14 * 24 * time.Hour), true
	case "month":
		return now.AddDate(0, int(n), 0), true
	case "year":
		return now.AddDate(int(n), 0, 0), true
	}
	return time.Time{}, false
}

func nextDayOfMonth(day int, timePart string, now time.Time) time.Time {
	candidate := time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, now.Location())
	resolved := withTime(candidate, timePart, now)
	if resolved.Before(now) {
		candidate = candidate.AddDate(0, 1, 0)
		resolved = withTime(candidate, timePart, now)
	}
	return resolved
}

func nextWeekday(target time.Weekday, now time.Time) time.Time {
	days := (int(target) - int(now.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	return now.AddDate(0, 0, days)
}

func withTime(day time.Time, timePart string, now time.Time) time.Time {
	if at, ok := timeOfDay("at "+timePart, day); ok && timePart != "" {
		return at
	}
	_ = now
	return dayAt(day, DefaultReminderHour, 0)
}

func dayAt(day time.Time, hour, minute int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, day.Location())
}

// splitAt divides "1 october at 9am" into its date and time halves.
func splitAt(s string) (string, string) {
	index := strings.Index(s, " at ")
	if index == -1 {
		return s, ""
	}
	return strings.TrimSpace(s[:index]), strings.TrimSpace(s[index+4:])
}

// timeOfDay reads "at HH:MM", "at 9am", "at 9.30pm" out of s and stamps
// it onto the given day.
func timeOfDay(s string, day time.Time) (time.Time, bool) {
	index := strings.Index(s, "at ")
	if index == -1 {
		return time.Time{}, false
	}
	value := strings.TrimSpace(s[index+3:])
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, ".", ":")
	if value == "noon" || value == "midday" {
		return dayAt(day, 12, 0), true
	}
	if value == "midnight" {
		return dayAt(day, 0, 0), true
	}
	for _, layout := range []string{"15:04", "15", "3:04pm", "3pm", "3:04am", "3am"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return dayAt(day, parsed.Hour(), parsed.Minute()), true
		}
	}
	return time.Time{}, false
}

// ordinal reads "1", "1st", "22nd", "2026" as a number.
func ordinal(field string) (int, bool) {
	trimmed := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(field, "st"), "nd"), "rd"), "th")
	n, err := strconv.Atoi(trimmed)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func firstWord(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
