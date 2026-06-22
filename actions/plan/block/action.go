// Package block — the AI-callable plan/block tool. The agent calls
// this from inside a plan-task execution when it cannot make progress
// (missing data, ambiguous instruction, external dependency
// unreachable). The action posts the reason to the API's
// /api/v1/internal/plan_task/:planTaskID/block endpoint which
// transitions the task to status='failed', records a task_blocked
// audit event, and pokes the plan so the next tick derives
// status='blocked'. Downstream tasks then never dispatch.
//
// Wiring notes:
//
//   - `plan_task_id` is pre-configured at `${flow.plan_task_id}` so
//     the AI never has to track the UUID across tool turns. The
//     editor + AI Prompt action treat ${flow.X} inputs as
//     pre-resolved and exclude them from the tool schema sent to the
//     model — the AI sees only `reason` as a tool parameter.
//   - `tool_result` is the first output by convention so the model
//     sees a readable summary regardless of outcome.
//   - 404 from the API surfaces as a failed `tool_result` so the AI
//     can self-correct (e.g. retry on the next tick). 200/idempotent
//     reports the same shape — the action doesn't punish duplicate
//     calls.
package block

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
	Name         = "Plan / Block"
	Description  = "Mark the current plan task as blocked when you cannot make progress. Call with a clear reason."
	Website      = "https://www.flomation.co"
	Icon         = "list-check+circle-stop"
	Date         = "22/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "plan_task_id",
		Type:        core.ConnectionTypeString,
		Label:       "Plan task identifier — defaults to the currently-running task via the Plan Task Trigger context",
		Required:    true,
		Placeholder: "${flow.plan_task_id}",
	},
	{
		Name:        "reason",
		Type:        core.ConnectionTypeText,
		Label:       "Clear, user-readable reason this task cannot make progress",
		Required:    true,
		Placeholder: "Missing API credentials for the BigQuery pull",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "plan_task_id", Type: core.ConnectionTypeString, Label: "Plan task ID"},
	{Name: "outcome", Type: core.ConnectionTypeString, Label: "blocked | idempotent | not_found"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// Execute is the action entry point invoked by the executor flow
// engine when the AI tool loop selects this tool.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return failResult("", "plan/block requires an agent context (APIURL must be set)"), nil
	}

	planTaskID := strInput("plan_task_id", inputs)
	reason := strInput("reason", inputs)
	if planTaskID == "" {
		return failResult("", "plan_task_id is required (Plan Task Trigger should populate ${flow.plan_task_id})"), nil
	}
	if reason == "" {
		return failResult(planTaskID, "reason is required — explain why the task cannot proceed"), nil
	}

	body, _ := json.Marshal(map[string]interface{}{
		"reason": reason,
	})

	url := fmt.Sprintf("%s/api/v1/internal/plan_task/%s/block", ctx.APIURL, planTaskID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return failResult(planTaskID, fmt.Sprintf("build request: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return failResult(planTaskID, fmt.Sprintf("plan/block API call failed: %v", err)), nil
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode == http.StatusNotFound {
		// The AI passed an unknown plan_task_id — likely a stale UUID
		// from an earlier turn. Surface verbatim so the model reads
		// "not found" and corrects.
		return failResult(planTaskID, fmt.Sprintf(
			"plan_task not found (HTTP 404): %s",
			truncate(string(respBody), 512))), nil
	}

	if resp.StatusCode != http.StatusOK {
		return failResult(planTaskID, fmt.Sprintf(
			"plan/block rejected (HTTP %d): %s",
			resp.StatusCode, truncate(string(respBody), 512))), nil
	}

	var result struct {
		PlanTaskID string `json:"plan_task_id"`
		Outcome    string `json:"outcome"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return failResult(planTaskID,
			fmt.Sprintf("plan/block succeeded but response decode failed: %v", err)), nil
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Plan task %s marked blocked: %s", result.PlanTaskID, truncate(reason, 256)),
		"plan_task_id": result.PlanTaskID,
		"outcome":      result.Outcome,
		"success":      true,
		"error":        "",
	}, nil
}

func strInput(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func failResult(planTaskID, msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result":  msg,
		"plan_task_id": planTaskID,
		"outcome":      "",
		"success":      false,
		"error":        msg,
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "… [truncated]"
}
