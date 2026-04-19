package format_date

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Format Date"
	Description  = "Format a date/time value or the current time into a string"
	Website      = "https://www.flomation.co"
	Icon         = "calendar"
	Date         = "19/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "datetime",
		Type:        core.ConnectionTypeString,
		Label:       "Date/Time",
		Placeholder: "Leave empty for current time",
	},
	{
		Name:  "input_format",
		Type:  core.ConnectionTypeString,
		Label: "Input Format",
		Options: []core.ConnectionOption{
			{Name: "ISO 8601 (2006-01-02T15:04:05Z07:00)", Value: "iso8601"},
			{Name: "RFC 2822 (Mon, 02 Jan 2006 15:04:05 -0700)", Value: "rfc2822"},
			{Name: "Unix Timestamp (seconds)", Value: "unix"},
			{Name: "Date Only (2006-01-02)", Value: "date"},
			{Name: "Custom", Value: "custom"},
		},
	},
	{
		Name:        "custom_input_format",
		Type:        core.ConnectionTypeString,
		Label:       "Custom Input Format (Go layout)",
		Placeholder: "2006-01-02 15:04:05",
	},
	{
		Name:  "output_format",
		Type:  core.ConnectionTypeString,
		Label: "Output Format",
		Options: []core.ConnectionOption{
			{Name: "ISO 8601 (2006-01-02T15:04:05Z07:00)", Value: "iso8601"},
			{Name: "RFC 2822 (Mon, 02 Jan 2006 15:04:05 -0700)", Value: "rfc2822"},
			{Name: "Date Only (2006-01-02)", Value: "date"},
			{Name: "Time Only (15:04:05)", Value: "time"},
			{Name: "Friendly (02 Jan 2006, 15:04)", Value: "friendly"},
			{Name: "Day/Month/Year (02/01/2006)", Value: "dmy"},
			{Name: "Month/Day/Year (01/02/2006)", Value: "mdy"},
			{Name: "Unix Timestamp", Value: "unix"},
			{Name: "Custom", Value: "custom"},
		},
	},
	{
		Name:        "custom_output_format",
		Type:        core.ConnectionTypeString,
		Label:       "Custom Output Format (Go layout)",
		Placeholder: "Monday, 02 January 2006",
	},
	{
		Name:        "timezone",
		Type:        core.ConnectionTypeString,
		Label:       "Timezone",
		Placeholder: "UTC (e.g. Europe/London, America/New_York)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "result", Type: core.ConnectionTypeString, Label: "Formatted Date"},
	{Name: "year", Type: core.ConnectionTypeInteger, Label: "Year"},
	{Name: "month", Type: core.ConnectionTypeInteger, Label: "Month"},
	{Name: "day", Type: core.ConnectionTypeInteger, Label: "Day"},
	{Name: "hour", Type: core.ConnectionTypeInteger, Label: "Hour"},
	{Name: "minute", Type: core.ConnectionTypeInteger, Label: "Minute"},
	{Name: "second", Type: core.ConnectionTypeInteger, Label: "Second"},
	{Name: "day_of_week", Type: core.ConnectionTypeString, Label: "Day of Week"},
	{Name: "unix_timestamp", Type: core.ConnectionTypeInteger, Label: "Unix Timestamp"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// inputLayouts maps input_format option values to Go time layouts.
var inputLayouts = map[string]string{
	"iso8601": time.RFC3339,
	"rfc2822": time.RFC1123Z,
	"date":    "2006-01-02",
}

// outputLayouts maps output_format option values to Go time layouts.
var outputLayouts = map[string]string{
	"iso8601":  time.RFC3339,
	"rfc2822":  time.RFC1123Z,
	"date":     "2006-01-02",
	"time":     "15:04:05",
	"friendly": "02 Jan 2006, 15:04",
	"dmy":      "02/01/2006",
	"mdy":      "01/02/2006",
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	datetimeStr := str("datetime", inputs)
	inputFmt := str("input_format", inputs)
	customInputFmt := str("custom_input_format", inputs)
	outputFmt := str("output_format", inputs)
	customOutputFmt := str("custom_output_format", inputs)
	tz := str("timezone", inputs)

	// Determine timezone
	loc := time.UTC
	if tz != "" {
		var err error
		loc, err = time.LoadLocation(tz)
		if err != nil {
			return fail("invalid timezone: " + tz), nil
		}
	}

	// Parse or use current time
	var t time.Time
	if datetimeStr == "" {
		t = time.Now().In(loc)
	} else {
		var err error
		t, err = parseInput(datetimeStr, inputFmt, customInputFmt)
		if err != nil {
			return fail("failed to parse datetime: " + err.Error()), nil
		}
		t = t.In(loc)
	}

	// Format output
	formatted, err := formatOutput(t, outputFmt, customOutputFmt)
	if err != nil {
		return fail(err.Error()), nil
	}

	return map[string]interface{}{
		"tool_result":    formatted,
		"result":         formatted,
		"year":           int64(t.Year()),
		"month":          int64(t.Month()),
		"day":            int64(t.Day()),
		"hour":           int64(t.Hour()),
		"minute":         int64(t.Minute()),
		"second":         int64(t.Second()),
		"day_of_week":    t.Weekday().String(),
		"unix_timestamp": t.Unix(),
		"success":        true,
		"error":          "",
	}, nil
}

func parseInput(value, format, customFormat string) (time.Time, error) {
	if format == "unix" {
		// Try parsing as integer seconds
		var seconds int64
		if _, err := fmt.Sscanf(value, "%d", &seconds); err != nil {
			return time.Time{}, fmt.Errorf("invalid unix timestamp: %s", value)
		}
		return time.Unix(seconds, 0), nil
	}

	if format == "custom" && customFormat != "" {
		return time.Parse(customFormat, value)
	}

	if layout, ok := inputLayouts[format]; ok {
		return time.Parse(layout, value)
	}

	// Auto-detect: try common formats
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC1123Z,
		time.RFC1123,
		"02/01/2006 15:04:05",
		"02/01/2006",
		"01/02/2006",
	} {
		if t, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", value)
}

func formatOutput(t time.Time, format, customFormat string) (string, error) {
	if format == "unix" {
		return fmt.Sprintf("%d", t.Unix()), nil
	}

	if format == "custom" && customFormat != "" {
		return t.Format(customFormat), nil
	}

	if layout, ok := outputLayouts[format]; ok {
		return t.Format(layout), nil
	}

	// Default to ISO 8601
	return t.Format(time.RFC3339), nil
}

func fail(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result":    "Error: " + msg,
		"result":         "",
		"year":           int64(0),
		"month":          int64(0),
		"day":            int64(0),
		"hour":           int64(0),
		"minute":         int64(0),
		"second":         int64(0),
		"day_of_week":    "",
		"unix_timestamp": int64(0),
		"success":        false,
		"error":          msg,
	}
}

func str(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
