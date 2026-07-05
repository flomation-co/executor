// Package list_identities retrieves all known channel identities for the
// current agent user. This helps the AI understand which channels a user
// is already linked on before offering a new link.
package list_identities

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
	Name         = "List Identities"
	Description  = "List all known channel identities for the current user"
	Website      = "https://www.flomation.co"
	Icon         = "user-group"
	Date         = "29/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID", Placeholder: "${flow.agent_id}", Required: true},
	{Name: "agent_user_id", Type: core.ConnectionTypeString, Label: "Agent User ID", Placeholder: "${flow.agent_user_id}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "identities", Type: core.ConnectionTypeObject, Label: "Identities (JSON array)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentID, err := requiredString("agent_id", inputs)
	if err != nil {
		return map[string]interface{}{"tool_result": "agent_id is required"}, fmt.Errorf("agent_id is required")
	}

	agentUserID, err := requiredString("agent_user_id", inputs)
	if err != nil {
		return map[string]interface{}{"tool_result": "agent_user_id is required"}, fmt.Errorf("agent_user_id is required")
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/identity?agent_user_id=%s",
		ctx.APIURL, agentID, agentUserID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		msg := fmt.Sprintf("Failed to list identities: %v", err)
		return map[string]interface{}{"tool_result": msg}, fmt.Errorf("API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := fmt.Sprintf("Failed to list identities: API returned %d: %s", resp.StatusCode, string(respBody))
		return map[string]interface{}{"tool_result": msg}, fmt.Errorf("%s", msg)
	}

	var identities []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&identities); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(identities) == 0 {
		return map[string]interface{}{
			"tool_result": "No linked identities found for this user.",
			"identities":  identities,
			"count":       0,
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("This user has %d linked identity/identities:\n", len(identities)))
	for _, id := range identities {
		channel, _ := id["channel_type"].(string)
		extID, _ := id["external_id"].(string)
		displayName, _ := id["display_name"].(string)

		sb.WriteString(fmt.Sprintf("\n- %s: %s", channel, extID))
		if displayName != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", displayName))
		}
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"identities":  identities,
		"count":       len(identities),
	}, nil
}

func requiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return *c.String(), nil
}
