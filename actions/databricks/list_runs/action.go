package databricks_list_runs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	databricks "flomation.app/automate/executor/actions/databricks"
)

const (
	Author       = "Flomation"
	Organisation = "Flomation"
	Name         = "Databricks List Runs"
	Description  = "List runs for a Databricks job, optionally filtered to active runs"
	Website      = "https://www.flomation.co"
	Icon         = "database+list"
	Date         = "24/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Workspace URL", Placeholder: "https://dbc-xxxxxxxx.cloud.databricks.com", Required: true},
	{Name: "token", Type: core.ConnectionTypeSecret, Label: "Access Token (PAT)", Placeholder: "dapi...", Required: true},
	{Name: "job_id", Type: core.ConnectionTypeInteger, Label: "Job ID", Placeholder: "Optional — omit to list runs across all jobs"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Default 20, max 25 (runs/list API cap)"},
	{Name: "active_only", Type: core.ConnectionTypeBoolean, Label: "Active Runs Only"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "runs", Type: core.ConnectionTypeObject, Label: "Runs (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "has_more", Type: core.ConnectionTypeBoolean, Label: "Has More"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type listRunsResponse struct {
	Runs    []map[string]interface{} `json:"runs"`
	HasMore bool                     `json:"has_more"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	host, token, err := databricks.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	if jobID, ok := databricks.OptionalInt("job_id", inputs); ok {
		query.Set("job_id", fmt.Sprintf("%d", jobID))
	}
	if limit, ok := databricks.OptionalInt("limit", inputs); ok && limit > 0 {
		// Limit cap is endpoint-specific and easy to confuse:
		//   - jobs/runs/list (this endpoint): max 25, default 20
		//   - jobs/list (listing jobs, NOT runs):  max 100
		// We call runs/list, so 25 is the ceiling. Clamp rather than let the API
		// reject the request, so an over-large limit degrades gracefully.
		const maxRunsListLimit = 25
		if limit > maxRunsListLimit {
			limit = maxRunsListLimit
		}
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	if conn := core.FindConnection("active_only", inputs); conn != nil && conn.Boolean() != nil && *conn.Boolean() {
		query.Set("active_only", "true")
	}

	path := "/api/2.1/jobs/runs/list"
	if encoded := query.Encode(); encoded != "" {
		path = path + "?" + encoded
	}

	resp, err := databricks.ExecuteAPI(host, token, http.MethodGet, path, nil)
	if err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to list runs: %s", err)), nil
	}
	if err := databricks.CheckResponse(resp); err != nil {
		return databricks.ErrorResult(err.Error()), nil
	}

	var out listRunsResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}
	if out.Runs == nil {
		out.Runs = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d run(s)", len(out.Runs)),
		"runs":        out.Runs,
		"count":       len(out.Runs),
		"has_more":    out.HasMore,
		"success":     true,
		"error":       "",
	}, nil
}
