package databricks_cancel_run

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	databricks "flomation.app/automate/executor/actions/databricks"
)

const (
	Author       = "Flomation"
	Organisation = "Flomation"
	Name         = "Databricks Cancel Run"
	Description  = "Cancel an in-progress Databricks job run"
	Website      = "https://www.flomation.co"
	Icon         = "database+ban"
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
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
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

	payload := map[string]interface{}{"run_id": runID}

	resp, err := databricks.ExecuteAPI(host, token, http.MethodPost, "/api/2.1/jobs/runs/cancel", payload)
	if err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to cancel run: %s", err)), nil
	}
	if err := databricks.CheckResponse(resp); err != nil {
		return databricks.ErrorResult(err.Error()), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Requested cancellation of run %d", runID),
		"success":     true,
		"error":       "",
	}, nil
}
