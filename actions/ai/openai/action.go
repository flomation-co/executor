package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	ai_common "flomation.app/automate/executor/actions/ai"
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
	{
		Name:        "conversation_history",
		Type:        core.ConnectionTypeObject,
		Label:       "Conversation History",
		Placeholder: "${conversation_history}",
	},
	{
		// TEMPORARY: tool definitions as JSON. Will be replaced by
		// automatic discovery from the tools subgraph wired to the
		// Tools handle.
		Name:        "tool_definitions",
		Type:        core.ConnectionTypeText,
		Label:       "Tool Definitions (JSON)",
		Placeholder: `[{"type":"function","function":{"name":"web_search","description":"Search the web","parameters":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}}]`,
	},
}

var Outputs = [...]core.Connection{
	{Name: "response", Type: core.ConnectionTypeString, Label: "Response"},
	{Name: "model", Type: core.ConnectionTypeString, Label: "Model Used"},
	{Name: "prompt_tokens", Type: core.ConnectionTypeInteger, Label: "Prompt Tokens"},
	{Name: "completion_tokens", Type: core.ConnectionTypeInteger, Label: "Completion Tokens"},
	{Name: "total_tokens", Type: core.ConnectionTypeInteger, Label: "Total Tokens"},
	{Name: "should_respond", Type: core.ConnectionTypeBoolean, Label: "Should Respond"},
	{Name: "tool_calls_count", Type: core.ConnectionTypeInteger, Label: "Tool Calls"},
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

	systemPromptStr := ""
	systemConn := core.FindConnection("system_prompt", inputs)
	if systemConn != nil && systemConn.String() != nil && *systemConn.String() != "" {
		systemPromptStr = *systemConn.String()
	}

	// Parse tool definitions if provided (OpenAI format)
	var tools []interface{}
	toolDefsConn := core.FindConnection("tool_definitions", inputs)
	if toolDefsConn != nil {
		var raw string
		if s := toolDefsConn.String(); s != nil && *s != "" {
			raw = *s
		}
		if raw != "" {
			_ = json.Unmarshal([]byte(raw), &tools)
		}
	}

	// Check if we're in a tool loop (re-invocation with tool results)
	var messages []interface{}
	toolCallsCount := 0

	if convState, ok := flow.GetVariable(core.ToolConversationStateKey); ok && convState != nil {
		if ms, ok := convState.([]interface{}); ok {
			messages = ms
		}

		// Append tool results
		if toolResults, ok := flow.GetVariable(core.ToolResultsKey); ok && toolResults != nil {
			if results, ok := toolResults.([]core.ToolResult); ok {
				for _, r := range results {
					messages = append(messages, map[string]interface{}{
						"role":         "tool",
						"tool_call_id": r.ToolUseID,
						"content":      r.Content,
					})
				}
			}
		}
	} else {
		// First invocation — build messages from system prompt + history + prompt
		if systemPromptStr != "" {
			messages = append(messages, map[string]interface{}{
				"role":    "system",
				"content": systemPromptStr,
			})
		}

		historyConn := core.FindConnection("conversation_history", inputs)
		if historyConn != nil {
			history := ai_common.ParseConversationHistory(historyConn.Value)
			if len(history) > 0 {
				history = ai_common.TruncateHistoryForBudget(
					history, systemPromptStr, prompt,
					int(maxTokens), ai_common.ModelContextWindow(model),
				)
				for _, m := range history {
					if m.Role == "" || m.Content == "" {
						continue
					}
					messages = append(messages, map[string]interface{}{
						"role":    m.Role,
						"content": m.Content,
					})
				}
			}
		}

		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": prompt,
		})
	}

	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
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
	defer func() { _ = resp.Body.Close() }()

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
				Content   *string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"` // JSON string
					} `json:"function"`
				} `json:"tool_calls,omitempty"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
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

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	choice := result.Choices[0]

	// Check for tool calls
	if choice.FinishReason == "tool_calls" && len(choice.Message.ToolCalls) > 0 {
		var toolRequests []core.ToolRequest

		// Build the assistant message with tool_calls for conversation state
		assistantMsg := map[string]interface{}{
			"role":       "assistant",
			"tool_calls": choice.Message.ToolCalls,
		}
		if choice.Message.Content != nil {
			assistantMsg["content"] = *choice.Message.Content
		}

		for _, tc := range choice.Message.ToolCalls {
			var input map[string]interface{}
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			toolRequests = append(toolRequests, core.ToolRequest{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}

		messages = append(messages, assistantMsg)

		out := map[string]interface{}{
			core.ToolRequestsKey:          toolRequests,
			core.ToolConversationStateKey: messages,
			"stop_reason":                 "tool_calls",
			"model":                       result.Model,
			"prompt_tokens":               result.Usage.PromptTokens,
			"completion_tokens":           result.Usage.CompletionTokens,
			"total_tokens":                result.Usage.TotalTokens,
			"tool_calls_count":            len(toolRequests),
			"success":                     true,
			"error":                       "",
		}
		// Capture any text the model emitted alongside tool calls so the
		// engine can send it to the user via the Response handle mid-loop.
		if choice.Message.Content != nil && *choice.Message.Content != "" {
			out[core.IntermediateTextKey] = *choice.Message.Content
		}
		return out, nil
	}

	// Final text response
	content := ""
	if choice.Message.Content != nil {
		content = *choice.Message.Content
	}

	shouldRespond := true
	trimmed := strings.TrimSpace(content)
	if trimmed == "[NO_RESPONSE]" || strings.Contains(trimmed, "[NO_RESPONSE]") {
		shouldRespond = false
		content = ""
	}

	if shouldRespond && content != "" {
		ai_common.RecordAssistantReply(flow.GoContext(), flow.GetContext(), content)
	}

	return map[string]interface{}{
		"response":          content,
		"should_respond":    shouldRespond,
		"model":             result.Model,
		"prompt_tokens":     result.Usage.PromptTokens,
		"completion_tokens": result.Usage.CompletionTokens,
		"total_tokens":      result.Usage.TotalTokens,
		"tool_calls_count":  toolCallsCount,
		"success":           true,
		"error":             "",
	}, nil
}