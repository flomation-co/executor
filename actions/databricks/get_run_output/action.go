package databricks_get_run_output

import (
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	databricks "flomation.app/automate/executor/actions/databricks"
)

const (
	Author       = "Flomation"
	Organisation = "Flomation"
	Name         = "Databricks Get Run Output"
	Description  = "Get the output of a completed Databricks job task run"
	Website      = "https://www.flomation.co"
	Icon         = "database+file-lines"
	Date         = "24/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Workspace URL", Placeholder: "https://dbc-xxxxxxxx.cloud.databricks.com", Required: true},
	{Name: "token", Type: core.ConnectionTypeSecret, Label: "Access Token (PAT)", Placeholder: "dapi...", Required: true},
	{Name: "run_id", Type: core.ConnectionTypeInteger, Label: "Task Run ID", Placeholder: "The run_id of a single task, not the job run", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "result", Type: core.ConnectionTypeString, Label: "Notebook Result"},
	{Name: "logs", Type: core.ConnectionTypeString, Label: "Logs"},
	{Name: "run_error", Type: core.ConnectionTypeString, Label: "Run Error"},
	{Name: "output", Type: core.ConnectionTypeObject, Label: "Output (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type runOutputResponse struct {
	NotebookOutput *struct {
		Result    string `json:"result"`
		Truncated bool   `json:"truncated"`
	} `json:"notebook_output"`
	Logs  string `json:"logs"`
	Error string `json:"error"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	host, token, err := databricks.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	runID, err := databricks.RequiredInt("run_id", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := databricks.ExecuteAPI(host, token, http.MethodGet, fmt.Sprintf("/api/2.1/jobs/runs/get-output?run_id=%d", runID), nil)
	if err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to get run output: %s", err)), nil
	}
	if err := databricks.CheckResponse(resp); err != nil {
		return databricks.ErrorResult(err.Error()), nil
	}

	var out runOutputResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	var raw map[string]interface{}
	_ = json.Unmarshal(resp.Body, &raw)

	notebookResult := ""
	if out.NotebookOutput != nil {
		notebookResult = out.NotebookOutput.Result
	}

	// run_error is distinct from the node-level "error" (which signals the API
	// call itself failed). get-output returns HTTP 200 with a populated `error`
	// when the API call succeeded but the underlying task failed (e.g. the
	// notebook threw), so run_error is meaningfully non-empty for failed runs and
	// empty for successful ones — it's the task's failure message, not ours.
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Fetched output for run %d", runID),
		"result":      notebookResult,
		"logs":        out.Logs,
		"run_error":   out.Error,
		"output":      raw,
		"success":     true,
		"error":       "",
	}, nil
}
