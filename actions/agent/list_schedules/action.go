// Package list_schedules retrieves all scheduled tasks for an agent.
package list_schedules

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Schedules"
	Description  = "List all scheduled tasks for the agent"
	Website      = "https://www.flomation.co"
	Icon         = "clock"
	Date         = "29/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID", Placeholder: "${flow.agent_id}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "schedules", Type: core.ConnectionTypeObject, Label: "Schedules (JSON array)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentID, err := requiredString("agent_id", inputs)
	if err != nil {
		return map[string]interface{}{"tool_result": "agent_id is required"}, fmt.Errorf("agent_id is required")
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/schedule", ctx.APIURL, agentID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return map[string]interface{}{"tool_result": fmt.Sprintf("Failed to list schedules: %v", err)}, fmt.Errorf("API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := fmt.Sprintf("Failed to list schedules: API returned %d: %s", resp.StatusCode, string(respBody))
		return map[string]interface{}{"tool_result": msg}, fmt.Errorf("%s", msg)
	}

	var schedules []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&schedules); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(schedules) == 0 {
		return map[string]interface{}{
			"tool_result": "No scheduled tasks found.",
			"schedules":   schedules,
			"count":       0,
		}, nil
	}

	// Build readable summary.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d scheduled task(s):\n", len(schedules)))
	for _, s := range schedules {
		name, _ := s["name"].(string)
		mode, _ := s["schedule_mode"].(string)
		enabled, _ := s["enabled"].(bool)
		desc, _ := s["description"].(string)
		tod, _ := s["time_of_day"].(string)
		tz, _ := s["timezone"].(string)

		status := "enabled"
		if !enabled {
			status = "disabled"
		}

		sb.WriteString(fmt.Sprintf("\n- %s [%s, %s]", name, mode, status))
		if tod != "" {
			sb.WriteString(fmt.Sprintf(" at %s", tod))
		}
		if tz != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", tz))
		}
		if desc != "" {
			sb.WriteString(fmt.Sprintf("\n  %s", desc))
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"schedules":   schedules,
		"count":       len(schedules),
	}, nil
}

func requiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return *c.String(), nil
}