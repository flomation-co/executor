// Package create_schedule creates a recurring scheduled task for an agent.
// The agent can set up daily, weekly, or interval-based schedules that
// fire the orchestrator flow on a timer.
package create_schedule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Schedule"
	Description  = "Create a recurring scheduled task for the agent"
	Website      = "https://www.flomation.co"
	Icon         = "clock+plus"
	Date         = "29/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID", Placeholder: "${flow.agent_id}", Required: true},
	{Name: "agent_user_id", Type: core.ConnectionTypeString, Label: "Agent User ID", Placeholder: "${flow.agent_user_id}"},
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID", Placeholder: "${flow.conversation_id}"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Schedule name", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "What to do when the schedule fires", Required: true},
	{
		Name: "schedule_mode", Type: core.ConnectionTypeString, Label: "Mode", Required: true,
		Options: []core.ConnectionOption{
			{Name: "Daily", Value: "daily"},
			{Name: "Weekly", Value: "weekly"},
			{Name: "Interval", Value: "interval"},
		},
	},
	{Name: "time_of_day", Type: core.ConnectionTypeString, Label: "Time of day (HH:MM, for daily/weekly)"},
	{Name: "days_of_week", Type: core.ConnectionTypeString, Label: "Days of week (comma-separated, for weekly)"},
	{Name: "interval_val", Type: core.ConnectionTypeString, Label: "Interval value (for interval mode)"},
	{
		Name: "unit", Type: core.ConnectionTypeString, Label: "Interval unit",
		Options: []core.ConnectionOption{
			{Name: "Minutes", Value: "minutes"},
			{Name: "Hours", Value: "hours"},
			{Name: "Days", Value: "days"},
		},
	},
	{Name: "timezone", Type: core.ConnectionTypeString, Label: "Timezone (IANA)", Placeholder: "Europe/London"},
	{Name: "source_channel", Type: core.ConnectionTypeString, Label: "Source channel type", Placeholder: "${flow.channel_type}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "schedule_id", Type: core.ConnectionTypeString, Label: "Schedule ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentID, err := requiredString("agent_id", inputs)
	if err != nil {
		return map[string]interface{}{"tool_result": "agent_id is required"}, fmt.Errorf("agent_id is required")
	}

	name, err := requiredString("name", inputs)
	if err != nil {
		return map[string]interface{}{"tool_result": "name is required"}, fmt.Errorf("name is required")
	}

	description, err := requiredString("description", inputs)
	if err != nil {
		return map[string]interface{}{"tool_result": "description is required"}, fmt.Errorf("description is required")
	}

	mode, err := requiredString("schedule_mode", inputs)
	if err != nil {
		return map[string]interface{}{"tool_result": "schedule_mode is required"}, fmt.Errorf("schedule_mode is required")
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	payload := map[string]interface{}{
		"name":          name,
		"description":   description,
		"schedule_mode": mode,
	}

	if v := optionalString("agent_user_id", inputs); v != "" {
		payload["agent_user_id"] = v
	}
	if v := optionalString("conversation_id", inputs); v != "" {
		payload["conversation_id"] = v
	}
	if v := optionalString("time_of_day", inputs); v != "" {
		payload["time_of_day"] = v
	}
	if v := optionalString("days_of_week", inputs); v != "" {
		payload["days_of_week"] = v
	}
	if v := optionalString("interval_val", inputs); v != "" {
		payload["interval_val"] = v
	}
	if v := optionalString("unit", inputs); v != "" {
		payload["unit"] = v
	}
	if v := optionalString("timezone", inputs); v != "" {
		payload["timezone"] = v
	}
	if v := optionalString("source_channel", inputs); v != "" {
		payload["source_channel"] = v
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/schedule", ctx.APIURL, agentID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return map[string]interface{}{"tool_result": fmt.Sprintf("Failed to create schedule: %v", err)}, fmt.Errorf("API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := fmt.Sprintf("Failed to create schedule: API returned %d: %s", resp.StatusCode, string(respBody))
		return map[string]interface{}{"tool_result": msg}, fmt.Errorf("%s", msg)
	}

	var result struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	summary := fmt.Sprintf("Schedule '%s' created successfully (%s mode)", name, mode)
	if mode == "daily" {
		if tod := optionalString("time_of_day", inputs); tod != "" {
			summary += fmt.Sprintf(" at %s", tod)
		}
	}
	if tz := optionalString("timezone", inputs); tz != "" {
		summary += fmt.Sprintf(" (%s)", tz)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"schedule_id": result.ID,
	}, nil
}

func requiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return *c.String(), nil
}

func optionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
