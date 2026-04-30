// Package fulfill_commitment is the executor action that marks an
// agent_commitment as fulfilled. Flow authors place this at the end of
// a wake-up branch in their orchestrator flow to explicitly mark the
// commitment as completed after the AI has produced its follow-up reply.
//
// The commitment poller in Launch (Phase 3) auto-marks commitments as
// fulfilled after a successful dispatch, so this action is optional for
// the common case. It exists for flow authors who want fine-grained
// control — e.g. only marking fulfilled after a conditional branch
// confirms the follow-up was actually useful, or after a delivery
// action confirms the message was sent to the channel.
package fulfill_commitment

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
	Name         = "Fulfil Commitment"
	Description  = "Mark an agent commitment as fulfilled after the follow-up has been delivered"
	Website      = "https://www.flomation.co"
	Icon         = "check-circle"
	Date         = "06/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "commitment_id",
		Type:        core.ConnectionTypeString,
		Label:       "Commitment ID",
		Placeholder: "${trigger.commitment_id}",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	commitmentIDConn := core.FindConnection("commitment_id", inputs)
	if commitmentIDConn == nil || commitmentIDConn.String() == nil || *commitmentIDConn.String() == "" {
		return nil, fmt.Errorf("commitment_id is required")
	}
	commitmentID := *commitmentIDConn.String()

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	body, _ := json.Marshal(map[string]string{"status": "fulfilled"})
	endpoint := fmt.Sprintf("%s/api/v1/internal/commitment/%s", ctx.APIURL, commitmentID)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPatch, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call fulfil endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return map[string]interface{}{
		"success": true,
	}, nil
}
