package databricks_invoke_model

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
	Name         = "Databricks Invoke Model"
	Description  = "Invoke a Databricks Model Serving endpoint (ML model or LLM) with a JSON payload"
	Website      = "https://www.flomation.co"
	Icon         = "brain+paper-plane"
	Date         = "25/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Workspace URL", Placeholder: "https://dbc-xxxxxxxx.cloud.databricks.com", Required: true},
	{Name: "token", Type: core.ConnectionTypeSecret, Label: "Access Token (PAT)", Placeholder: "dapi...", Required: true},
	{Name: "endpoint_name", Type: core.ConnectionTypeString, Label: "Serving Endpoint Name", Placeholder: "my-model-endpoint", Required: true},
	{Name: "payload", Type: core.ConnectionTypeText, Label: "Request Payload (JSON)", Placeholder: `{"messages":[{"role":"user","content":"Hello"}]} or {"dataframe_records":[{"x":1}]}`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "response", Type: core.ConnectionTypeObject, Label: "Response (JSON)"},
	{Name: "predictions", Type: core.ConnectionTypeObject, Label: "Predictions"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Chat Content"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	host, token, err := databricks.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	endpoint, err := databricks.RequiredString("endpoint_name", inputs)
	if err != nil {
		return nil, err
	}

	// payload stays a Text field so a user can paste literal JSON in the editor.
	// But the Connection value may also arrive already-structured (a map/slice)
	// when an upstream node's Object output is wired straight in — handle both,
	// so structured wiring doesn't require a manual stringify step.
	payloadConn := core.FindConnection("payload", inputs)
	if payloadConn == nil || payloadConn.Value == nil {
		return databricks.ErrorResult("payload is required"), nil
	}
	var payload interface{}
	switch v := payloadConn.Value.(type) {
	case string:
		if v == "" {
			return databricks.ErrorResult("payload is required"), nil
		}
		if err := json.Unmarshal([]byte(v), &payload); err != nil {
			return databricks.ErrorResult(fmt.Sprintf("payload is not valid JSON: %s", err)), nil
		}
	default:
		payload = v // already-structured value from an upstream node
	}

	path := "/serving-endpoints/" + url.PathEscape(endpoint) + "/invocations"
	resp, err := databricks.ExecuteAPI(host, token, http.MethodPost, path, payload)
	if err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to invoke endpoint: %s", err)), nil
	}
	if err := databricks.CheckResponse(resp); err != nil {
		return databricks.ErrorResult(err.Error()), nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(resp.Body, &raw); err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	// Smart-extract the two common response shapes so downstream nodes can wire
	// straight to a port without reaching into the raw object:
	//   - ML models:   {"predictions": [...]}
	//   - chat/LLM:    {"choices": [{"message": {"content": "..."}}]}  (OpenAI-style)
	predictions := raw["predictions"]
	content := extractChatContent(raw)

	summary := fmt.Sprintf("Invoked %s", endpoint)
	switch {
	case content != "":
		summary = fmt.Sprintf("%s — returned chat content", endpoint)
	case predictions != nil:
		summary = fmt.Sprintf("%s — returned predictions", endpoint)
	}

	return map[string]interface{}{
		"tool_result": summary,
		"response":    raw,
		"predictions": predictions,
		"content":     content,
		"success":     true,
		"error":       "",
	}, nil
}

// extractChatContent pulls choices[0].message.content from an OpenAI-style
// response, returning "" if the shape isn't present.
func extractChatContent(raw map[string]interface{}) string {
	choices, ok := raw["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return ""
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return ""
	}
	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return ""
	}
	content, _ := message["content"].(string)
	return content
}
