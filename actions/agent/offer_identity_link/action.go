// Package offer_identity_link creates an identity_link pending action
// offering to link the current user to an identity on another channel.
// This replaces the [LINK_OFFER:channel:id] tag mechanism with a
// direct tool call that gives the AI immediate feedback.
package offer_identity_link

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
	Name         = "Offer Identity Link"
	Description  = "Offer to link this user to their identity on another channel"
	Website      = "https://www.flomation.co"
	Icon         = "link"
	Date         = "29/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID", Placeholder: "${flow.agent_id}", Required: true},
	{Name: "agent_user_id", Type: core.ConnectionTypeString, Label: "Agent User ID", Placeholder: "${flow.agent_user_id}", Required: true},
	{
		Name: "target_channel", Type: core.ConnectionTypeString, Label: "Target channel type", Required: true,
		Options: []core.ConnectionOption{
			{Name: "Slack", Value: "slack"},
			{Name: "Telegram", Value: "telegram"},
			{Name: "Email", Value: "email"},
		},
	},
	{Name: "target_identifier", Type: core.ConnectionTypeString, Label: "Target identifier (username, email, etc.)", Required: true},
	{Name: "evidence", Type: core.ConnectionTypeString, Label: "Why this link is being offered", Required: true},
	{Name: "source_channel", Type: core.ConnectionTypeString, Label: "Source channel type", Placeholder: "${flow.channel_type}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "pending_action_id", Type: core.ConnectionTypeString, Label: "Pending Action ID"},
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

	targetChannel, err := requiredString("target_channel", inputs)
	if err != nil {
		return map[string]interface{}{"tool_result": "target_channel is required"}, fmt.Errorf("target_channel is required")
	}

	targetID, err := requiredString("target_identifier", inputs)
	if err != nil {
		return map[string]interface{}{"tool_result": "target_identifier is required"}, fmt.Errorf("target_identifier is required")
	}

	evidence, err := requiredString("evidence", inputs)
	if err != nil {
		return map[string]interface{}{"tool_result": "evidence is required"}, fmt.Errorf("evidence is required")
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	// Check if an open identity_link PA already exists for this user.
	matchURL := fmt.Sprintf("%s/api/v1/internal/agent/%s/pending-action/match?agent_user_id=%s&type=identity_link",
		ctx.APIURL, agentID, agentUserID)
	matchReq, _ := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, matchURL, nil)
	if ctx.Token != "" {
		matchReq.Header.Set("Authorization", "Bearer "+ctx.Token)
	}
	matchResp, err := ctx.InternalClient().Do(matchReq)
	if err == nil {
		defer func() { _ = matchResp.Body.Close() }()
		if matchResp.StatusCode == http.StatusOK {
			return map[string]interface{}{
				"tool_result": "An identity link offer is already pending for this user. They need to confirm or decline the existing one first.",
			}, nil
		}
	}

	// Create the identity_link pending action.
	payload := map[string]interface{}{
		"type":           "identity_link",
		"agent_user_id":  agentUserID,
		"evidence":       evidence,
		"source_channel": optionalString("source_channel", inputs),
		"payload": map[string]string{
			"channel_type": targetChannel,
			"external_id":  targetID,
		},
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/pending-action", ctx.APIURL, agentID)
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
		msg := fmt.Sprintf("Failed to create identity link offer: %v", err)
		return map[string]interface{}{"tool_result": msg}, fmt.Errorf("API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := fmt.Sprintf("Failed to create identity link offer: API returned %d: %s", resp.StatusCode, string(respBody))
		return map[string]interface{}{"tool_result": msg}, fmt.Errorf("%s", msg)
	}

	var result struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	summary := fmt.Sprintf("Identity link offer created. The user will be asked to confirm they also use %s as '%s'. Once confirmed, a verification message will be sent to that channel.",
		targetChannel, targetID)

	return map[string]interface{}{
		"tool_result":       summary,
		"pending_action_id": result.ID,
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