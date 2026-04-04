package write_state

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
	Name         = "Write Agent State"
	Description  = "Write a persistent state value to an agent's key-value store"
	Website      = "https://www.flomation.co"
	Icon         = "floppy-disk"
	Date         = "03/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "agent_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent ID",
		Placeholder: "ID of the agent",
		Required:    true,
	},
	{
		Name:        "key",
		Type:        core.ConnectionTypeString,
		Label:       "State Key",
		Placeholder: "conversation_history",
		Required:    true,
	},
	{
		Name:        "value",
		Type:        core.ConnectionTypeObject,
		Label:       "Value",
		Placeholder: "",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentIDConn := core.FindConnection("agent_id", inputs)
	if agentIDConn == nil || agentIDConn.String() == nil || *agentIDConn.String() == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	agentID := *agentIDConn.String()

	keyConn := core.FindConnection("key", inputs)
	if keyConn == nil || keyConn.String() == nil || *keyConn.String() == "" {
		return nil, fmt.Errorf("key is required")
	}
	key := *keyConn.String()

	valueConn := core.FindConnection("value", inputs)
	if valueConn == nil {
		return nil, fmt.Errorf("value is required")
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	payload, err := json.Marshal(map[string]interface{}{
		"value": valueConn.Value,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal value: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/state/%s", ctx.APIURL, agentID, key)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to write agent state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return map[string]interface{}{
		"success": true,
	}, nil
}
