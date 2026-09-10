// Package set_reminder lets an agent schedule its own follow-up during
// the turn it promises one, instead of having the promise reconstructed
// from its prose afterwards.
//
// The distinction matters. Extraction runs after the reply has been
// sent, so an agent that writes "I'll check back on Friday the 13th" is
// narrating a decision nothing has made yet — and on live, every
// commitment a model dated itself was wrong by a year and fired the
// moment it was written. Here the platform resolves the timing, stores
// it, and hands the resolved date back in the tool result, so the agent
// can tell the user a date that is already true.
package set_reminder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
	agent_actions "flomation.app/automate/executor/actions/agent"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Set Reminder"
	Description  = "Schedule a follow-up. The platform resolves the timing and returns the exact date it stored."
	Summary      = "Let your agent set a reminder and get the real date back"
	Website      = "https://www.flomation.co"
	Icon         = "bell+plus"
	Date         = "10/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:  "when",
		Type:  core.ConnectionTypeString,
		Label: "When to fire, in plain English: a duration (\"in 72 hours\", \"in 30 minutes\"), a relative day (\"tomorrow at 9am\", \"next Monday\"), or a date (\"1 October\", \"25 December at 09:00\"). Never work the calendar date out yourself — pass what was asked for and use the date this returns.",
		Placeholder: "in 72 hours",
		Required:    true,
	},
	{
		Name:        "description",
		Type:        core.ConnectionTypeString,
		Label:       "What to remind about, written so it still makes sense on its own days later",
		Placeholder: "Pull the updated campaign numbers and compare the click-to-view ratio",
		Required:    true,
	},
	{
		Name:  "kind",
		Type:  core.ConnectionTypeString,
		Label: "What sort of follow-up this is",
		Options: []core.ConnectionOption{
			{Name: "Reminder", Value: "reminder"},
			{Name: "Follow-up", Value: "followup"},
			{Name: "Monitor", Value: "monitor"},
			{Name: "Chase", Value: "chase"},
		},
		Value: "reminder",
	},
	{
		Name:  "recurrence",
		Type:  core.ConnectionTypeString,
		Label: "Leave empty for a one-off. Set it only when the follow-up was asked for repeatedly",
		Options: []core.ConnectionOption{
			{Name: "One-off", Value: ""},
			{Name: "Daily", Value: "daily"},
			{Name: "Weekdays", Value: "weekdays"},
			{Name: "Weekly", Value: "weekly"},
			{Name: "Monthly", Value: "monthly"},
		},
	},
	{
		Name:        "agent_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent ID",
		Placeholder: "${flow.agent_id}",
		Value:       "${flow.agent_id}",
		Required:    true,
	},
	{
		Name:        "agent_user_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent User ID",
		Placeholder: "${flow.agent_user_id}",
		Value:       "${flow.agent_user_id}",
	},
	{
		Name:        "conversation_id",
		Type:        core.ConnectionTypeString,
		Label:       "Conversation ID",
		Placeholder: "${flow.conversation_id}",
		Value:       "${flow.conversation_id}",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Tool result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "commitment_id", Type: core.ConnectionTypeString, Label: "Commitment ID"},
	{Name: "due_at", Type: core.ConnectionTypeString, Label: "Due at (ISO-8601)"},
	{Name: "due_at_friendly", Type: core.ConnectionTypeString, Label: "Due at, spelled out"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	when := stringInput("when", inputs)
	description := stringInput("description", inputs)
	agentID := stringInput("agent_id", inputs)

	if when == "" || description == "" {
		return failure("Both a time and a description are needed to set a reminder."), nil
	}
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	due, ok := agent_actions.ResolveWhen(when, time.Now())
	if !ok {
		return failure(fmt.Sprintf(
			"%q is not a time I can schedule. Say it as a duration (\"in 2 hours\"), a relative day "+
				"(\"tomorrow at 9am\", \"next Monday\") or a date (\"1 October at 09:00\"), then call this again. "+
				"No reminder has been set.", when)), nil
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	body := map[string]interface{}{
		"kind":         orDefault(stringInput("kind", inputs), "reminder"),
		"description":  description,
		"trigger_type": "time_elapsed",
		"due_at":       due.UTC().Format(time.RFC3339),
		"made_by":      "assistant",
	}
	if userID := stringInput("agent_user_id", inputs); userID != "" {
		body["agent_user_id"] = userID
	}
	if conversationID := stringInput("conversation_id", inputs); conversationID != "" {
		body["conversation_id"] = conversationID
		body["source_conversation"] = conversationID
	}
	if recurrence := stringInput("recurrence", inputs); recurrence != "" {
		body["recurrence"] = recurrence
	}

	commitmentID, err := post(flow, ctx, fmt.Sprintf("%s/api/v1/internal/agent/%s/commitment", ctx.APIURL, agentID), body)
	if err != nil {
		return failure(fmt.Sprintf("The reminder could not be stored: %v. Nothing is scheduled.", err)), nil
	}

	friendly := due.UTC().Format("Monday 2 January 2006 at 15:04 MST")
	return map[string]interface{}{
		"tool_result": fmt.Sprintf(
			"Reminder stored for %s (%s): %s. Tell the user that date exactly as written here — it is what the "+
				"platform will act on. Do not work out your own date or day of the week.",
			friendly, due.UTC().Format(time.RFC3339), description),
		"success":         true,
		"commitment_id":   commitmentID,
		"due_at":          due.UTC().Format(time.RFC3339),
		"due_at_friendly": friendly,
	}, nil
}

func failure(message string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result":     message,
		"success":         false,
		"commitment_id":   "",
		"due_at":          "",
		"due_at_friendly": "",
	}
}

func post(flow *core.Flow, ctx *core.ExecutionContext, endpoint string, body map[string]interface{}) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("the API returned %d: %s", resp.StatusCode, string(raw))
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		return "", nil
	}
	return created.ID, nil
}

func stringInput(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
