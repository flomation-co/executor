// Package delete_schedule removes a scheduled task by name.
package delete_schedule

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Delete Schedule"
	Description  = "Delete a scheduled task by name"
	Website      = "https://www.flomation.co"
	Icon         = "clock+trash"
	Date         = "29/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID", Placeholder: "${flow.agent_id}", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Schedule name to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
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

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	apiURL := fmt.Sprintf("%s/api/v1/internal/agent/%s/schedule/by-name/%s",
		ctx.APIURL, agentID, url.PathEscape(name))
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodDelete, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		msg := fmt.Sprintf("Failed to delete schedule '%s': %v", name, err)
		return map[string]interface{}{"tool_result": msg}, fmt.Errorf("API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := fmt.Sprintf("Failed to delete schedule '%s': API returned %d: %s", name, resp.StatusCode, string(respBody))
		return map[string]interface{}{"tool_result": msg}, fmt.Errorf("%s", msg)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Schedule '%s' deleted successfully.", name),
	}, nil
}

func requiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return *c.String(), nil
}