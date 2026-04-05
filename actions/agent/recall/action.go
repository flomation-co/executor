// Package recall is the executor action that reads memories from the
// agent_memory table via the API's Phase 2a internal list endpoint.
//
// This is the counterpart to agent/remember: it gives flow authors
// explicit access to a user's memories for cases where the automatic
// system-prompt assembly in Launch isn't enough (e.g. composing a
// summary, showing memories to the user, branching on whether a
// specific memory type exists).
package recall

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Recall Memories"
	Description  = "Fetch an agent's memories about a specific user"
	Website      = "https://www.flomation.co"
	Icon         = "brain"
	Date         = "05/04/2026"
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
		Name:        "pinned_only",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Pinned only",
		Placeholder: "false",
		Required:    false,
	},
	{
		Name:        "limit",
		Type:        core.ConnectionTypeInteger,
		Label:       "Limit",
		Placeholder: "20",
		Required:    false,
	},
}

var Outputs = [...]core.Connection{
	{Name: "memories", Type: core.ConnectionTypeObject, Label: "Memories"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentID, err := requiredString("agent_id", inputs)
	if err != nil {
		return nil, err
	}
	agentUserID, err := requiredString("agent_user_id", inputs)
	if err != nil {
		return nil, err
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	q := url.Values{}
	q.Set("agent_user_id", agentUserID)
	if optionalBool("pinned_only", inputs) {
		q.Set("pinned", "true")
	}
	if limit := optionalInt("limit", inputs); limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}

	endpoint := fmt.Sprintf("%s/api/v1/internal/agent/%s/memory?%s",
		ctx.APIURL, agentID, q.Encode())

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call recall endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Decode into []map[string]interface{} rather than a typed struct so
	// flows can reference memory fields via ${node.memories[0].title}
	// without the executor needing to know about the api.AgentMemory
	// type. This matches the pattern used by read_state, which also
	// returns opaque maps.
	var memories []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&memories); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return map[string]interface{}{
		"memories": memories,
		"count":    len(memories),
	}, nil
}

func requiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return *c.String(), nil
}

func optionalBool(name string, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return false
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	if s := c.String(); s != nil {
		return *s == "true"
	}
	return false
}

func optionalInt(name string, inputs []*core.Connection) int {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return 0
	}
	if i := c.Number(); i != nil {
		return int(*i)
	}
	if s := c.String(); s != nil {
		if n, err := strconv.Atoi(*s); err == nil {
			return n
		}
	}
	return 0
}
