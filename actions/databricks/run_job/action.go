package databricks_run_job

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
	Name         = "Databricks Run Job"
	Description  = "Trigger a Databricks job to run now and return the run ID"
	Website      = "https://www.flomation.co"
	Icon         = "database+play"
	Date         = "24/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Workspace URL", Placeholder: "https://dbc-xxxxxxxx.cloud.databricks.com", Required: true},
	{Name: "token", Type: core.ConnectionTypeSecret, Label: "Access Token (PAT)", Placeholder: "dapi...", Required: true},
	{Name: "job_id", Type: core.ConnectionTypeInteger, Label: "Job ID", Required: true},
	{Name: "parameters", Type: core.ConnectionTypeKeyValueArray, Label: "Job Parameters"},
	{Name: "idempotency_token", Type: core.ConnectionTypeString, Label: "Idempotency Token", Placeholder: "Optional — dedupe duplicate triggers"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "run_id", Type: core.ConnectionTypeInteger, Label: "Run ID"},
	{Name: "number_in_job", Type: core.ConnectionTypeInteger, Label: "Run Number"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type runNowResponse struct {
	RunID       int64 `json:"run_id"`
	NumberInJob int64 `json:"number_in_job"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	host, token, err := databricks.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	jobID, err := databricks.RequiredInt("job_id", inputs)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{"job_id": jobID}

	if conn := core.FindConnection("parameters", inputs); conn != nil {
		if pairs := conn.KeyValuePairs(); len(pairs) > 0 {
			params := make(map[string]string, len(pairs))
			for _, p := range pairs {
				params[p.Key] = p.Value
			}
			payload["job_parameters"] = params
		}
	}
	if tok := databricks.OptionalString("idempotency_token", inputs); tok != "" {
		payload["idempotency_token"] = tok
	}

	resp, err := databricks.ExecuteAPI(host, token, http.MethodPost, "/api/2.1/jobs/run-now", payload)
	if err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to trigger job: %s", err)), nil
	}
	if err := databricks.CheckResponse(resp); err != nil {
		return databricks.ErrorResult(err.Error()), nil
	}

	var out runNowResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Triggered job %d — run %d", jobID, out.RunID),
		"run_id":        out.RunID,
		"number_in_job": out.NumberInJob,
		"success":       true,
		"error":         "",
	}, nil
}
