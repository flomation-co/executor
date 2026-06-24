// Package cancel — the AI-callable plan/cancel tool. Lets the agent
// stop a plan it (or another agent it has access to) created, when
// the user changes their mind or the AI decides the plan is no
// longer useful.
//
// Symmetric with plan/block: same fail-clean envelope, same
// InternalClient() posture, same idempotent outcome handling. The
// main difference is that cancel operates at the PLAN level
// (cascades all pending + in_progress tasks), whereas block stops
// a single task.
//
// Wiring notes:
//
//   - `plan_id` is an AI-provided input (NOT pre-resolved from
//     ${flow.plan_id}). The AI may legitimately cancel a plan
//     OTHER than the one it's currently running inside (e.g. a
//     plan it created earlier that's now stale). Forcing the
//     AI to provide the ID makes that case work cleanly.
//   - `reason` is optional. If the AI calls cancel without one
//     the plan's cancelled_reason ends up empty — same posture
//     as a user clicking the cancel button without typing.
package cancel

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
	Name         = "Plan / Cancel"
	Description  = "Cancel a plan you created. Stops all pending and in-progress tasks. Idempotent on already-terminal plans."
	Website      = "https://www.flomation.co"
	Icon         = "list-check+circle-xmark"
	Date         = "23/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:     "plan_id",
		Type:     core.ConnectionTypeString,
		Label:    "Plan identifier — the UUID returned by plan/create or surfaced via plan/get_status",
		Required: true,
	},
	{
		Name:        "reason",
		Type:        core.ConnectionTypeText,
		Label:       "Optional human-readable reason recorded on the plan's cancelled_reason column",
		Required:    false,
		Placeholder: "user changed their mind",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "plan_id", Type: core.ConnectionTypeString, Label: "Plan ID"},
	{Name: "outcome", Type: core.ConnectionTypeString, Label: "cancelled | idempotent | not_found"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" || ctx.AgentID == "" {
		return failResult("", "plan/cancel requires an agent context (APIURL + AgentID)"), nil
	}

	planID := strInput("plan_id", inputs)
	reason := strInput("reason", inputs)
	if planID == "" {
		return failResult("", "plan_id is required"), nil
	}

	body, _ := json.Marshal(map[string]interface{}{
		"reason": reason,
	})

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/plan/%s/cancel",
		ctx.APIURL, ctx.AgentID, planID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return failResult(planID, fmt.Sprintf("build request: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return failResult(planID, fmt.Sprintf("plan/cancel API call failed: %v", err)), nil
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode == http.StatusNotFound {
		// Surface as a clean tool_result so the model can self-
		// correct (e.g. re-call plan/get_status to find the right
		// plan_id).
		return failResult(planID, fmt.Sprintf(
			"plan not found (HTTP 404): %s",
			truncate(string(respBody), 512))), nil
	}

	if resp.StatusCode != http.StatusOK {
		return failResult(planID, fmt.Sprintf(
			"plan/cancel rejected (HTTP %d): %s",
			resp.StatusCode, truncate(string(respBody), 512))), nil
	}

	var result struct {
		PlanID  string `json:"plan_id"`
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return failResult(planID,
			fmt.Sprintf("plan/cancel succeeded but response decode failed: %v", err)), nil
	}

	summary := fmt.Sprintf("Plan %s cancelled", result.PlanID)
	if result.Outcome == "idempotent" {
		summary = fmt.Sprintf("Plan %s was already terminal (no change)", result.PlanID)
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
