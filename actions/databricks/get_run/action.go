package databricks_get_run

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
	Name         = "Databricks Get Run"
	Description  = "Get the status and details of a Databricks job run"
	Website      = "https://www.flomation.co"
	Icon         = "database+magnifying-glass"
	Date         = "24/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Workspace URL", Placeholder: "https://dbc-xxxxxxxx.cloud.databricks.com", Required: true},
	{Name: "token", Type: core.ConnectionTypeSecret, Label: "Access Token (PAT)", Placeholder: "dapi...", Required: true},
	{Name: "run_id", Type: core.ConnectionTypeInteger, Label: "Run ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "life_cycle_state", Type: core.ConnectionTypeString, Label: "Life Cycle State"},
	{Name: "result_state", Type: core.ConnectionTypeString, Label: "Result State"},
	{Name: "run_page_url", Type: core.ConnectionTypeString, Label: "Run Page URL"},
	{Name: "run", Type: core.ConnectionTypeObject, Label: "Run (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type runResponse struct {
	State struct {
		LifeCycleState string `json:"life_cycle_state"`
		ResultState    string `json:"result_state"`
		StateMessage   string `json:"state_message"`
	} `json:"state"`
	RunPageURL string `json:"run_page_url"`
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

	resp, err := databricks.ExecuteAPI(host, token, http.MethodGet, fmt.Sprintf("/api/2.1/jobs/runs/get?run_id=%d", runID), nil)
	if err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to get run: %s", err)), nil
	}
	if err := databricks.CheckResponse(resp); err != nil {
		return databricks.ErrorResult(err.Error()), nil
	}

	var run runResponse
	if err := json.Unmarshal(resp.Body, &run); err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	var raw map[string]interface{}
	_ = json.Unmarshal(resp.Body, &raw)

	summary := fmt.Sprintf("Run %d — %s", runID, run.State.LifeCycleState)
	if run.State.ResultState != "" {
		summary = fmt.Sprintf("%s (%s)", summary, run.State.ResultState)
	}

	return map[string]interface{}{
		"tool_result":      summary,
		"life_cycle_state": run.State.LifeCycleState,
		"result_state":     run.State.ResultState,
		"run_page_url":     run.RunPageURL,
		"run":              raw,
		"success":          true,
		"error":            "",
	}, nil
}
