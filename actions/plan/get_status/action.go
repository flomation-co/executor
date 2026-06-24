// Package get_status — the AI-callable plan/get_status tool. Lets
// the agent introspect a plan's current state without waiting for
// a writeback or relying on parent-output substitution. Returns
// the same {plan, tasks} shape M2's read endpoint provides, plus
// a one-line tool_result summary the AI can read quickly.
//
// Typical use: the AI calls plan/create, then later wants to check
// "is that plan done yet?" before deciding what to do next. Or
// before calling plan/cancel — confirm the plan is still active
// rather than already terminal.
package get_status

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
	Name         = "Plan / Get Status"
	Description  = "Read a plan's current state: status, task counts, and full task list."
	Website      = "https://www.flomation.co"
	Icon         = "list-check+magnifying-glass"
	Date         = "23/06/2026"
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
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "One-line summary the AI reads first"},
	{Name: "plan_id", Type: core.ConnectionTypeString, Label: "Plan ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "active | completed | blocked | cancelled | draft"},
	{Name: "task_count", Type: core.ConnectionTypeInteger, Label: "Total tasks in the plan"},
	{Name: "completed_count", Type: core.ConnectionTypeInteger, Label: "Tasks with status=completed"},
	{Name: "failed_count", Type: core.ConnectionTypeInteger, Label: "Tasks with status=failed or cancelled"},
	{Name: "plan_json", Type: core.ConnectionTypeText, Label: "Full {plan, tasks} payload as JSON for AI drill-down"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// planResponse mirrors the API's wire shape for the read endpoint.
// Defined inline rather than imported to keep the action self-
// contained — the executor doesn't depend on the api package.
type planResponse struct {
	Plan struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	} `json:"plan"`
	Tasks []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"tasks"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" || ctx.AgentID == "" {
		return failResult("", "plan/get_status requires an agent context (APIURL + AgentID)"), nil
	}

	planID := strInput("plan_id", inputs)
	if planID == "" {
		return failResult("", "plan_id is required"), nil
	}

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/plan/%s",
		ctx.APIURL, ctx.AgentID, planID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, url, nil)
	if err != nil {
		return failResult(planID, fmt.Sprintf("build request: %v", err)), nil
	}

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return failResult(planID, fmt.Sprintf("plan/get_status API call failed: %v", err)), nil
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))

	if resp.StatusCode == http.StatusNotFound {
		return failResult(planID, fmt.Sprintf(
			"plan not found (HTTP 404): %s",
			truncate(string(respBody), 512))), nil
	}
	if resp.StatusCode != http.StatusOK {
		return failResult(planID, fmt.Sprintf(
			"plan/get_status rejected (HTTP %d): %s",
			resp.StatusCode, truncate(string(respBody), 512))), nil
	}

	var parsed planResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return failResult(planID,
			fmt.Sprintf("plan/get_status response decode failed: %v", err)), nil
	}

	// Compute the histogram. failed_count rolls cancelled in
	// because both represent "this task is not going to complete"
	// from the AI's planning perspective.
	completed, failed := 0, 0
	for _, t := range parsed.Tasks {
		switch t.Status {
		case "completed":
			completed++
		case "failed", "cancelled":
			failed++
		}
	}

	summary := fmt.Sprintf("Plan %q is %s — %d/%d tasks completed",
		parsed.Plan.Title, parsed.Plan.Status, completed, len(parsed.Tasks))
	if failed > 0 {
		summary += fmt.Sprintf(" (%d failed/cancelled)", failed)
	}

	return map[string]interface{}{
		"tool_result":     summary,
		"plan_id":         parsed.Plan.ID,
		"status":          parsed.Plan.Status,
		"task_count":      len(parsed.Tasks),
		"completed_count": completed,
		"failed_count":    failed,
		"plan_json":       string(respBody),
		"success":         true,
		"error":           "",
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
		"tool_result":     msg,
		"plan_id":         planID,
		"status":          "",
		"task_count":      0,
		"completed_count": 0,
		"failed_count":    0,
		"plan_json":       "",
		"success":         false,
		"error":           msg,
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "… [truncated]"
}
