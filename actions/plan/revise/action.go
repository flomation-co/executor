// Package revise — the AI-callable plan/revise tool. M5 closes the
// plan lifecycle by letting the AI (or a user via API) modify a
// plan's task graph: add tasks, remove tasks, update existing
// task fields. Allowed on draft, blocked, and active plans —
// terminal plans (completed, cancelled) are rejected.
//
// Wire shape on the API side is { add_tasks, remove_tasks,
// update_tasks } — see persistence.RevisionOps. The executor
// action takes each as a JSON-text input so the AI can construct
// them with the same shape it learned for plan/create.
//
// Common patterns:
//
//   * Add a new task: add_tasks = [{ "name": ..., "description":
//     ..., "depends_on": ["other_task_name"] }]
//   * Remove a failed task: remove_tasks = ["failed_task_name"]
//   * Tweak a pending task: update_tasks = [{ "name": "...",
//     "description": "clarified" }]
package revise

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
	Name         = "Plan / Revise"
	Description  = "Modify a plan's task graph: add, remove, or update tasks. Allowed on draft, blocked, and active plans."
	Website      = "https://www.flomation.co"
	Icon         = "list-check+pen"
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
	{
		Name:        "add_tasks",
		Type:        core.ConnectionTypeText,
		Label:       "Optional JSON array of new tasks to add. Each must have `name` and `description`; can also include `depends_on`, `inputs`, `flow_id` + `flow_revision_id`",
		Required:    false,
		Placeholder: `[{"name":"new_step","description":"do the thing","depends_on":["existing_task"]}]`,
	},
	{
		Name:        "remove_tasks",
		Type:        core.ConnectionTypeText,
		Label:       "Optional JSON array of task names to remove. Only pending, failed, and cancelled tasks can be removed",
		Required:    false,
		Placeholder: `["task_to_remove"]`,
	},
	{
		Name:        "update_tasks",
		Type:        core.ConnectionTypeText,
		Label:       "Optional JSON array of partial updates. Each must have `name`; provide only the fields you want to change",
		Required:    false,
		Placeholder: `[{"name":"existing_task","description":"clarified instructions"}]`,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "plan_id", Type: core.ConnectionTypeString, Label: "Plan ID"},
	{Name: "outcome", Type: core.ConnectionTypeString, Label: "revised | invalid | terminal | not_found"},
	{Name: "new_status", Type: core.ConnectionTypeString, Label: "Plan status after revise"},
	{Name: "added_count", Type: core.ConnectionTypeInteger, Label: "Number of tasks added"},
	{Name: "removed_count", Type: core.ConnectionTypeInteger, Label: "Number of tasks removed"},
	{Name: "updated_count", Type: core.ConnectionTypeInteger, Label: "Number of tasks updated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" || ctx.AgentID == "" {
		return failResult("", "plan/revise requires an agent context (APIURL + AgentID)"), nil
	}

	planID := strInput("plan_id", inputs)
	if planID == "" {
		return failResult("", "plan_id is required"), nil
	}

	// Build the request body by parsing each optional input. Empty
	// inputs become nil/empty in the body — the API rejects an
	// all-empty revise with a clear "empty_revision" 400.
	body := map[string]interface{}{}

	if raw := strInput("add_tasks", inputs); raw != "" {
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &arr); err != nil {
			return failResult(planID, fmt.Sprintf("add_tasks must be a JSON array: %v", err)), nil
		}
		body["add_tasks"] = arr
	}
	if raw := strInput("remove_tasks", inputs); raw != "" {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err != nil {
			return failResult(planID, fmt.Sprintf("remove_tasks must be a JSON array of strings: %v", err)), nil
		}
		body["remove_tasks"] = arr
	}
	if raw := strInput("update_tasks", inputs); raw != "" {
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &arr); err != nil {
			return failResult(planID, fmt.Sprintf("update_tasks must be a JSON array: %v", err)), nil
		}
		body["update_tasks"] = arr
	}

	if len(body) == 0 {
		return failResult(planID, "provide at least one of add_tasks, remove_tasks, update_tasks"), nil
	}

	bodyBytes, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/plan/%s/revise",
		ctx.APIURL, ctx.AgentID, planID)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return failResult(planID, fmt.Sprintf("build request: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return failResult(planID, fmt.Sprintf("plan/revise API call failed: %v", err)), nil
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode == http.StatusNotFound {
		return failResult(planID, fmt.Sprintf(
			"plan not found (HTTP 404): %s", truncate(string(respBody), 512))), nil
	}
	if resp.StatusCode == http.StatusConflict {
		return failResult(planID, fmt.Sprintf(
			"plan is terminal and cannot be revised: %s", truncate(string(respBody), 512))), nil
	}
	if resp.StatusCode == http.StatusBadRequest {
		// Surface the validator's structured detail verbatim so
		// the AI can read e.g. {"reason":"cycle","task_name":"a"}
		// and self-correct.
		return failResult(planID, fmt.Sprintf(
			"plan/revise validation failed: %s", truncate(string(respBody), 512))), nil
	}
	if resp.StatusCode != http.StatusOK {
		return failResult(planID, fmt.Sprintf(
			"plan/revise rejected (HTTP %d): %s",
			resp.StatusCode, truncate(string(respBody), 512))), nil
	}

	var result struct {
		PlanID     string   `json:"plan_id"`
		Outcome    string   `json:"outcome"`
		NewStatus  string   `json:"new_status"`
		AddedIDs   []string `json:"added_ids"`
		RemovedIDs []string `json:"removed_ids"`
		UpdatedIDs []string `json:"updated_ids"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return failResult(planID,
			fmt.Sprintf("plan/revise succeeded but response decode failed: %v", err)), nil
	}

	addedN := len(result.AddedIDs)
	removedN := len(result.RemovedIDs)
	updatedN := len(result.UpdatedIDs)
	summary := fmt.Sprintf("Plan %s revised — %d added, %d removed, %d updated (status: %s)",
		result.PlanID, addedN, removedN, updatedN, result.NewStatus)

	return map[string]interface{}{
		"tool_result":   summary,
		"plan_id":       result.PlanID,
		"outcome":       result.Outcome,
		"new_status":    result.NewStatus,
		"added_count":   addedN,
		"removed_count": removedN,
		"updated_count": updatedN,
		"success":       true,
		"error":         "",
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
		"tool_result":   msg,
		"plan_id":       planID,
		"outcome":       "",
		"new_status":    "",
		"added_count":   0,
		"removed_count": 0,
		"updated_count": 0,
		"success":       false,
		"error":         msg,
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "… [truncated]"
}
