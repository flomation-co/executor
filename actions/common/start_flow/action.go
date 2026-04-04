package start_flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Start Flow"
	Description  = "Trigger another flow, optionally waiting for completion"
	Website      = "https://www.flomation.co"
	Icon         = "share-from-square"
	Date         = "25/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "flow_id",
		Type:        core.ConnectionTypeString,
		Label:       "Flow ID",
		Placeholder: "ID of the flow to trigger",
		Required:    true,
	},
	{
		Name:  "wait_for_completion",
		Type:  core.ConnectionTypeBoolean,
		Label: "Wait for Completion",
	},
	{
		Name:        "timeout_seconds",
		Type:        core.ConnectionTypeInteger,
		Label:       "Timeout (seconds)",
		Placeholder: "300",
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "execution_id",
		Type:  core.ConnectionTypeString,
		Label: "Execution ID",
	},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
	},
	{
		Name:  "result",
		Type:  core.ConnectionTypeObject,
		Label: "Flow Result",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	flowIDConn := core.FindConnection("flow_id", inputs)
	if flowIDConn == nil || flowIDConn.String() == nil || *flowIDConn.String() == "" {
		return nil, fmt.Errorf("flow_id is required")
	}
	flowID := *flowIDConn.String()

	waitForCompletion := false
	waitConn := core.FindConnection("wait_for_completion", inputs)
	if waitConn != nil && waitConn.String() != nil {
		waitForCompletion = *waitConn.String() == "true"
	}

	timeoutSeconds := int64(300)
	timeoutConn := core.FindConnection("timeout_seconds", inputs)
	if timeoutConn != nil && timeoutConn.String() != nil {
		if v, err := strconv.ParseInt(*timeoutConn.String(), 10, 64); err == nil && v > 0 {
			timeoutSeconds = v
		}
	}

	// Get API URL from execution context
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("API URL not available in execution context")
	}

	// Trigger the flow via internal endpoint (no auth required)
	triggerURL := fmt.Sprintf("%s/api/v1/internal/flo/%s/execute", ctx.APIURL, flowID)
	req, err := http.NewRequest(http.MethodPost, triggerURL, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to trigger flow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to trigger flow (status %d): %s", resp.StatusCode, string(body))
	}

	var triggerResult struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&triggerResult); err != nil {
		return nil, fmt.Errorf("failed to parse trigger response: %w", err)
	}

	executionID := triggerResult.ID
	status := "queued"
	var result interface{}

	if waitForCompletion {
		deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
		pollURL := fmt.Sprintf("%s/api/v1/internal/execution/%s", ctx.APIURL, executionID)

		for time.Now().Before(deadline) {
			time.Sleep(2 * time.Second)

			pollReq, _ := http.NewRequest(http.MethodGet, pollURL, nil)
			if ctx.Token != "" {
				pollReq.Header.Set("Authorization", "Bearer "+ctx.Token)
			}
			pollResp, err := client.Do(pollReq)
			if err != nil {
				log.WithFields(log.Fields{"error": err}).Warn("failed to poll execution status")
				continue
			}

			var execData struct {
				ExecutionStatus  string      `json:"execution_status"`
				CompletionStatus string      `json:"completion_status"`
				Result           interface{} `json:"result"`
			}
			if err := json.NewDecoder(pollResp.Body).Decode(&execData); err != nil {
				pollResp.Body.Close()
				continue
			}
			pollResp.Body.Close()

			if execData.ExecutionStatus == "executed" {
				status = execData.CompletionStatus
				result = execData.Result
				break
			}

			status = execData.ExecutionStatus
		}

		if status == "queued" || status == "running" || status == "allocated" {
			status = "timeout"
		}
	}

	return map[string]interface{}{
		"execution_id": executionID,
		"status":       status,
		"result":       result,
	}, nil
}
