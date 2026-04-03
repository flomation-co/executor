package read_state

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Read Agent State"
	Description  = "Read a persistent state value from an agent's key-value store"
	Website      = "https://www.flomation.co"
	Icon         = "database"
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
}

var Outputs = [...]core.Connection{
	{Name: "value", Type: core.ConnectionTypeObject, Label: "State Value"},
	{Name: "exists", Type: core.ConnectionTypeBoolean, Label: "Key Exists"},
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

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/state/%s", ctx.APIURL, agentID, key)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return map[string]interface{}{
			"value":  nil,
			"exists": false,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return map[string]interface{}{
		"value":  result["state_value"],
		"exists": true,
	}, nil
}
