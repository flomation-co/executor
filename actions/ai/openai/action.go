package openai

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
	Name         = "OpenAI Prompt"
	Description  = "Send a prompt to the OpenAI Chat Completions API and return the response"
	Website      = "https://www.flomation.co"
	Icon         = "brain"
	Date         = "04/04/2026"
	Type         = core.ActionTypeAction

	defaultModel     = "gpt-4o"
	defaultMaxTokens = 2048
	apiURL           = "https://api.openai.com/v1/chat/completions"
	maxResponseBody  = 1 << 20 // 1 MB
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeString,
		Label:       "API Key",
		Placeholder: "sk-...",
		Required:    true,
	},
	{
		Name:  "model",
		Type:  core.ConnectionTypeString,
		Label: "Model",
		Options: []core.ConnectionOption{
			{Name: "GPT-4o", Value: "gpt-4o"},
			{Name: "GPT-4o Mini", Value: "gpt-4o-mini"},
			{Name: "GPT-4.1", Value: "gpt-4.1"},
			{Name: "GPT-4.1 Mini", Value: "gpt-4.1-mini"},
			{Name: "GPT-4.1 Nano", Value: "gpt-4.1-nano"},
			{Name: "o3", Value: "o3"},
			{Name: "o3 Mini", Value: "o3-mini"},
			{Name: "o4 Mini", Value: "o4-mini"},
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
	{Name: "prompt_tokens", Type: core.ConnectionTypeInteger, Label: "Prompt Tokens"},
	{Name: "completion_tokens", Type: core.ConnectionTypeInteger, Label: "Completion Tokens"},
	{Name: "total_tokens", Type: core.ConnectionTypeInteger, Label: "Total Tokens"},
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

	// Build messages
	var messages []map[string]string

	systemConn := core.FindConnection("system_prompt", inputs)
	if systemConn != nil && systemConn.String() != nil && *systemConn.String() != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": *systemConn.String(),
		})
	}

	messages = append(messages, map[string]string{
		"role":    "user",
		"content": prompt,
	})

	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
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
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
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
			errMsg = string(respBody)
		}
		return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, errMsg)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	content := ""
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}

	return map[string]interface{}{
		"response":          content,
		"model":             result.Model,
		"prompt_tokens":     result.Usage.PromptTokens,
		"completion_tokens": result.Usage.CompletionTokens,
		"total_tokens":      result.Usage.TotalTokens,
		"success":           true,
		"error":             "",
	}, nil
}
