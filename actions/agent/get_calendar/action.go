// Package get_calendar is the executor action that returns the
// agent_user's upcoming Google Calendar events. The agent calls it
// as a tool when the conversation makes it relevant — when the
// user mentions where they need to be, asks about their day,
// suggests they're running late, etc.
//
// This deliberately is NOT part of the system prompt. The agent
// decides when calendar awareness matters; token cost stays at zero
// until the model actually invokes the tool.
//
// Returns an empty list with no_calendar=true when the user hasn't
// linked a Google Calendar — the model can then offer to set up
// the connection rather than silently fail.
package get_calendar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Calendar"
	Description  = "Return the user's upcoming Google Calendar events. Use when the conversation hints at scheduling — where they are, where they need to be, whether they're running late."
	Website      = "https://www.flomation.co"
	Icon         = "calendar+magnifying-glass"
	Date         = "20/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "agent_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent ID",
		Placeholder: "${flow.agent_id}",
		Required:    true,
	},
	{
		Name:        "agent_user_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent User ID",
		Placeholder: "${flow.agent_user_id}",
		Required:    true,
	},
	{
		Name:        "hours",
		Type:        core.ConnectionTypeInteger,
		Label:       "How many hours ahead to look (default 24, max 168).",
		Placeholder: "24",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "events", Type: core.ConnectionTypeObject, Label: "Upcoming events"},
	{Name: "event_count", Type: core.ConnectionTypeInteger, Label: "Number of events returned"},
	{Name: "no_calendar", Type: core.ConnectionTypeBoolean, Label: "True if the user hasn't linked a Google Calendar"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// Execute calls the API's internal endpoint over mTLS. The
// credential lookup, Google fetch, and per-user cache all live
// server-side — this action is a thin pass-through.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentID := optionalString("agent_id", inputs)
	if agentID == "" {
		return errResult("agent_id is required")
	}
	agentUserID := optionalString("agent_user_id", inputs)
	if agentUserID == "" {
		return errResult("agent_user_id is required")
	}
	if strings.HasPrefix(agentUserID, "${") {
		return errResult("agent_user_id contains an unresolved variable reference; wire ${flow.agent_user_id}")
	}
	hours := optionalInt("hours", inputs)
	if hours <= 0 {
		hours = 24
	}
	if hours > 168 {
		hours = 168
	}

	execCtx := flow.GetContext()
	if execCtx == nil || execCtx.APIURL == "" {
		return errResult("execution context with API URL is required")
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"agent_user_id": agentUserID,
		"hours":         hours,
	})
	endpoint := fmt.Sprintf("%s/api/v1/internal/agent/%s/calendar/events", execCtx.APIURL, agentID)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return errResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := execCtx.InternalClient().Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("calendar fetch failed: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return errResult(fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)))
	}

	var parsed struct {
		Events     []map[string]interface{} `json:"events"`
		EventCount int                      `json:"event_count"`
		NoCalendar bool                     `json:"no_calendar"`
		Error      string                   `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return errResult(fmt.Sprintf("failed to decode response: %v", err))
	}

	log.WithFields(log.Fields{
		"agent_id":      agentID,
		"agent_user_id": agentUserID,
		"hours":         hours,
		"event_count":   parsed.EventCount,
		"no_calendar":   parsed.NoCalendar,
	}).Info("agent/get_calendar fetched calendar events")

	var summary string
	switch {
	case parsed.NoCalendar:
		summary = "The user has not linked a Google Calendar to this agent yet. Offer to help them connect one if calendar awareness would be useful."
	case parsed.EventCount == 0:
		summary = fmt.Sprintf("No events in the next %d hours.", hours)
	default:
		summary = fmt.Sprintf("Found %d events in the next %d hours.", parsed.EventCount, hours)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"events":      parsed.Events,
		"event_count": parsed.EventCount,
		"no_calendar": parsed.NoCalendar,
		"success":     parsed.Error == "",
		"error":       parsed.Error,
	}, nil
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"events":      []map[string]interface{}{},
		"event_count": 0,
		"no_calendar": false,
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

func optionalInt(name string, inputs []*core.Connection) int {
	c := core.FindConnection(name, inputs)
	if c == nil || c.Number() == nil {
		return 0
	}
	return int(*c.Number())
}
