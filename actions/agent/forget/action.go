// Package forget is the executor action that deletes a memory from the
// agent_memory table via the API's Phase 2a internal delete endpoint.
//
// Used by flow authors who want to explicitly remove a memory after a
// user correction, and by the Phase 2d extraction pipeline when the
// extracted `forget_memory` pending action is confirmed.
package forget

import (
	"fmt"
	"io"
	"net/http"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Forget Memory"
	Description  = "Delete a specific memory from an agent's store"
	Website      = "https://www.flomation.co"
	Icon         = "brain"
	Date         = "05/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "memory_id",
		Type:        core.ConnectionTypeString,
		Label:       "Memory ID",
		Placeholder: "UUID of the memory to delete",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	memoryIDConn := core.FindConnection("memory_id", inputs)
	if memoryIDConn == nil || memoryIDConn.String() == nil || *memoryIDConn.String() == "" {
		return nil, fmt.Errorf("memory_id is required")
	}
	memoryID := *memoryIDConn.String()

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	endpoint := fmt.Sprintf("%s/api/v1/internal/memory/%s", ctx.APIURL, memoryID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodDelete, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call forget endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// The API returns 204 on success. Any other status is a failure we
	// surface to the flow author — unlike read operations, a silent
	// forget failure would leave the user thinking the memory is gone
	// when it is not. That's worse than a loud error.
	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return map[string]interface{}{
		"success": true,
	}, nil
}
