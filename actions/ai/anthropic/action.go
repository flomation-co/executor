package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Anthropic Prompt"
	Description  = "Send a prompt to the Anthropic Messages API and return the response"
	Website      = "https://www.flomation.co"
	Icon         = "brain"
	Date         = "04/04/2026"
	Type         = core.ActionTypeAction

	defaultModel     = "claude-sonnet-4-6"
	defaultMaxTokens = 2048
	apiURL           = "https://api.anthropic.com/v1/messages"
	apiVersion       = "2023-06-01"
	maxResponseBody  = 1 << 20 // 1 MB
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeString,
		Label:       "API Key",
		Placeholder: "sk-ant-...",
		Required:    true,
	},
	{
		Name:  "model",
		Type:  core.ConnectionTypeString,
		Label: "Model",
		Options: []core.ConnectionOption{
			{Name: "Claude Sonnet 4.6", Value: "claude-sonnet-4-6"},
			{Name: "Claude Haiku 4.5", Value: "claude-haiku-4-5-20251001"},
			{Name: "Claude Opus 4.6", Value: "claude-opus-4-6"},
		},
	},
	{
		Name:        "system_prompt",
		Type:        core.ConnectionTypeText,
		Label:       "System Prompt",
		Placeholder: "You are a helpful assistant.",
	},
	{
		Name:        "prompt",
		Type:        core.ConnectionTypeText,
		Label:       "Prompt",
		Placeholder: "What would you like to ask?",
		Required:    true,
	},
	{
		Name:        "max_tokens",
		Type:        core.ConnectionTypeInteger,
		Label:       "Max Tokens",
		Placeholder: "2048",
	},
	{
		Name:        "temperature",
		Type:        core.ConnectionTypeString,
		Label:       "Temperature",
		Placeholder: "0.7",
	},
}

var Outputs = [...]core.Connection{
	{Name: "response", Type: core.ConnectionTypeString, Label: "Response"},
	{Name: "model", Type: core.ConnectionTypeString, Label: "Model Used"},
	{Name: "input_tokens", Type: core.ConnectionTypeInteger, Label: "Input Tokens"},
	{Name: "output_tokens", Type: core.ConnectionTypeInteger, Label: "Output Tokens"},
	{Name: "stop_reason", Type: core.ConnectionTypeString, Label: "Stop Reason"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKeyConn := core.FindConnection("api_key", inputs)
	if apiKeyConn == nil || apiKeyConn.String() == nil || *apiKeyConn.String() == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	apiKey := *apiKeyConn.String()

	promptConn := core.FindConnection("prompt", inputs)
	if promptConn == nil || promptConn.String() == nil || *promptConn.String() == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	prompt := *promptConn.String()

	model := defaultModel
	modelConn := core.FindConnection("model", inputs)
	if modelConn != nil && modelConn.String() != nil && *modelConn.String() != "" {
		model = *modelConn.String()
	}

	maxTokens := int64(defaultMaxTokens)
	maxTokensConn := core.FindConnection("max_tokens", inputs)
	if maxTokensConn != nil && maxTokensConn.Number() != nil && *maxTokensConn.Number() > 0 {
		maxTokens = *maxTokensConn.Number()
	}

	temperature := 0.7
	tempConn := core.FindConnection("temperature", inputs)
	if tempConn != nil && tempConn.String() != nil && *tempConn.String() != "" {
		fmt.Sscanf(*tempConn.String(), "%f", &temperature)
	}

	// Build request
	payload := map[string]interface{}{
		"model":       model,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	// System prompt is a top-level field in the Anthropic API
	systemConn := core.FindConnection("system_prompt", inputs)
	if systemConn != nil && systemConn.String() != nil && *systemConn.String() != "" {
		payload["system"] = *systemConn.String()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"response": "", "model": model, "success": false, "error": err.Error(),
			"input_tokens": int64(0), "output_tokens": int64(0), "stop_reason": "",
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(respBody, &apiErr)
		errMsg := apiErr.Error.Message
		if errMsg == "" {
			errMsg = fmt.Sprintf("API returned %d", resp.StatusCode)
		}
		return map[string]interface{}{
			"response": "", "model": model, "success": false, "error": errMsg,
			"input_tokens": int64(0), "output_tokens": int64(0), "stop_reason": "",
		}, nil
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return map[string]interface{}{
			"response": "", "model": model, "success": false, "error": "failed to parse response",
			"input_tokens": int64(0), "output_tokens": int64(0), "stop_reason": "",
		}, nil
	}

	// Extract text content
	var content string
	for _, block := range result.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return map[string]interface{}{
		"response":      content,
		"model":         result.Model,
		"input_tokens":  result.Usage.InputTokens,
		"output_tokens": result.Usage.OutputTokens,
		"stop_reason":   result.StopReason,
		"success":       true,
		"error":         "",
	}, nil
}
