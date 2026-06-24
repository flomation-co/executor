// Package start — the AI-callable plan/start tool. M4 introduced
// draft-by-default for plan/create: the agent authors a plan,
// summarises it to the user, and waits for explicit approval before
// calling plan/start to transition the plan from draft to active
// (which kicks off task dispatch).
//
// Symmetric with plan/cancel and plan/block: same fail-clean
// envelope, same InternalClient() posture. Body is empty — the URL
// path's planID identifies the target; the POST verb triggers the
// transition.
package start

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
	Name         = "Plan / Start"
	Description  = "Start a draft plan you previously authored. Transitions the plan from draft to active and begins dispatching tasks."
	Website      = "https://www.flomation.co"
	Icon         = "list-check+play"
	Date         = "24/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:     "plan_id",
		Type:     core.ConnectionTypeString,
		Label:    "Plan identifier — the UUID returned by plan/create",
		Required: true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "plan_id", Type: core.ConnectionTypeString, Label: "Plan ID"},
	{Name: "outcome", Type: core.ConnectionTypeString, Label: "started | idempotent | not_found | already_terminal"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" || ctx.AgentID == "" {
		return failResult("", "plan/start requires an agent context (APIURL + AgentID)"), nil
	}

	planID := strInput("plan_id", inputs)
	if planID == "" {
		return failResult("", "plan_id is required"), nil
	}

	// Body is empty — but we still send Content-Type and a {} body
	// for consistency with the rest of the plan/* tool family. The
	// API's gin handler tolerates either.
	body := []byte("{}")
	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/plan/%s/start",
		ctx.APIURL, ctx.AgentID, planID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return failResult(planID, fmt.Sprintf("build request: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return failResult(planID, fmt.Sprintf("plan/start API call failed: %v", err)), nil
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode == http.StatusNotFound {
		return failResult(planID, fmt.Sprintf(
			"plan not found (HTTP 404): %s",
			truncate(string(respBody), 512))), nil
	}

	if resp.StatusCode == http.StatusConflict {
		// Plan is already terminal (completed/cancelled). Surface
		// clearly so the AI knows the plan cannot be started — it
		// should NOT silently retry.
		return failResult(planID, fmt.Sprintf(
			"plan is already terminal and cannot be started: %s",
			truncate(string(respBody), 512))), nil
	}

	if resp.StatusCode != http.StatusOK {
		return failResult(planID, fmt.Sprintf(
			"plan/start rejected (HTTP %d): %s",
			resp.StatusCode, truncate(string(respBody), 512))), nil
	}

	var result struct {
		PlanID  string `json:"plan_id"`
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return failResult(planID,
			fmt.Sprintf("plan/start succeeded but response decode failed: %v", err)), nil
	}

	summary := fmt.Sprintf("Plan %s started — task dispatch beginning", result.PlanID)
	if result.Outcome == "idempotent" {
		summary = fmt.Sprintf("Plan %s was already active (no change)", result.PlanID)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"plan_id":     result.PlanID,
		"outcome":     result.Outcome,
		"success":     true,
		"error":       "",
	}, nil
}

func strInput(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func failResult(planID, msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"plan_id":     planID,
		"outcome":     "",
		"success":     false,
		"error":       msg,
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "… [truncated]"
}
