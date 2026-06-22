// Package create — the AI-callable plan/create tool. The agent uses
// this when a user asks for something that genuinely benefits from a
// multi-step structured plan (rather than a single immediate reply).
// The plan is AI-authored at runtime; the executor merely hands the
// (title, goal, tasks) tuple to the API's mTLS-only
// /api/v1/internal/agent/:id/plan endpoint and surfaces the
// response.
//
// AI-tool wiring notes:
//
//   - `tool_result` is the first output by convention so the model
//     sees a readable summary regardless of whether the call
//     succeeded.
//   - Inputs are minimal — the agent supplies title, goal, and a
//     JSON-encoded tasks array. The API does all the validation
//     (cycle detection, ref resolution, flow_revision verification)
//     and returns structured 400s the action surfaces verbatim so
//     the model can self-correct.
//   - The action requires an agent context: ctx.APIURL,
//     ctx.AgentID, plus the mTLS-configured InternalClient. Without
//     these the call would either 401 at the API or get rejected
//     by the executor's URL routing — fail fast with a clear
//     tool_result instead.
package create

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
	Name         = "Create Plan"
	Description  = "Generate a multi-step plan with dependent tasks the agent will autonomously progress over time."
	Website      = "https://www.flomation.co"
	Icon         = "list-check"
	Date         = "22/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "title",
		Type:        core.ConnectionTypeString,
		Label:       "Short plan title — shown to the user and on the plan-detail view",
		Required:    true,
		Placeholder: "Q3 review",
	},
	{
		Name:        "goal",
		Type:        core.ConnectionTypeText,
		Label:       "Goal statement — what success looks like once every task completes",
		Required:    true,
		Placeholder: "Pull this quarter's metrics, summarise for leadership, send for sign-off.",
	},
	{
		Name: "tasks_json",
		Type: core.ConnectionTypeText,
		Label: "Tasks (JSON array). Each task requires `name` + `description`; optional `inputs`, `depends_on`. " +
			"Omit `flow_id` to let the agent's orchestrator handle the task (recommended for most work). " +
			"Specify `flow_id` + `flow_revision_id` together to pin a curated flow when determinism matters.",
		Required:    true,
		Placeholder: `[{"name":"pull","description":"pull this quarter's metrics","inputs":{"quarter":"Q3"}}]`,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "plan_id", Type: core.ConnectionTypeString, Label: "Plan ID"},
	{Name: "task_count", Type: core.ConnectionTypeInteger, Label: "Tasks created"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Plan status"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// Execute is the action entry point invoked by the executor flow
// engine when the AI tool loop selects this tool.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" || ctx.AgentID == "" {
		return failResult("plan/create requires an agent context (APIURL + AgentID)"), nil
	}

	title := strInput("title", inputs)
	goal := strInput("goal", inputs)
	tasksJSON := strInput("tasks_json", inputs)
	if title == "" || goal == "" || tasksJSON == "" {
		return failResult("title, goal, and tasks_json are all required"), nil
	}

	// Decode tasks here so a malformed array fails with a clean
	// tool_result rather than a confusing API 400 about JSON shape.
	var tasks []map[string]interface{}
	if err := json.Unmarshal([]byte(tasksJSON), &tasks); err != nil {
		return failResult(fmt.Sprintf("tasks_json must be a JSON array: %v", err)), nil
	}
	if len(tasks) == 0 {
		return failResult("tasks_json must contain at least one task"), nil
	}

	body, _ := json.Marshal(map[string]interface{}{
		"title":                   title,
		"goal":                    goal,
		"tasks":                   tasks,
		"owner_user_id":           nonEmpty(ctx.UserID),
		"organisation_id":         nonEmpty(ctx.OrganisationID),
		"created_by_execution_id": nonEmpty(ctx.ExecutionID),
	})

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/plan", ctx.APIURL, ctx.AgentID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return failResult(fmt.Sprintf("build request: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return failResult(fmt.Sprintf("plan/create API call failed: %v", err)), nil
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode != http.StatusCreated {
		// Surface the structured error body verbatim — the validator
		// returns task_name + reason so the model can self-correct
		// (e.g. "duplicate_task_name, task_name=ingest" → pick a
		// distinct name and retry).
		return failResult(fmt.Sprintf("plan/create rejected (HTTP %d): %s",
			resp.StatusCode, truncate(string(respBody), 512))), nil
	}

	var result struct {
		PlanID    string `json:"plan_id"`
		TaskCount int    `json:"task_count"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return failResult(fmt.Sprintf("plan/create succeeded but response decode failed: %v", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Plan %q created with %d tasks (status: %s)",
			title, result.TaskCount, result.Status),
		"plan_id":    result.PlanID,
		"task_count": result.TaskCount,
		"status":     result.Status,
		"success":    true,
		"error":      "",
	}, nil
}

// nonEmpty returns the string verbatim, or empty-string if input is
// empty. Kept inline as a marker — the API treats missing optional
// fields as absent, but explicit "" works the same in JSON. The
// helper is here in case future versions need a sentinel pointer.
func nonEmpty(s string) string {
	return s
}

func strInput(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func failResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"plan_id":     "",
		"task_count":  0,
		"status":      "failed",
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
