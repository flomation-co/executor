package openwebui

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
	Name         = "Open WebUI Prompt"
	Description  = "Send a prompt to an Open WebUI (or any OpenAI-compatible) Chat Completions endpoint and return the response"
	Website      = "https://www.flomation.co"
	Icon         = "brain+globe"
	Date         = "11/06/2026"
	Type         = core.ActionTypeAction

	defaultMaxTokens = 2048
	chatPath         = "/api/chat/completions"
	maxResponseBody  = 1 << 20 // 1 MB
)

var Inputs = [...]core.Connection{
	{
		Name:        "endpoint",
		Type:        core.ConnectionTypeString,
		Label:       "Endpoint",
		Placeholder: "https://openwebui.example.com",
		Required:    true,
	},
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Key",
		Placeholder: "sk-...",
		Required:    true,
	},
	{
		// Open WebUI exposes arbitrary, deployment-specific model IDs
		// (whatever the operator has connected), so this is free text
		// rather than a fixed dropdown. Find available IDs at
		// {endpoint}/api/models or in the Open WebUI model picker.
		Name:        "model",
		Type:        core.ConnectionTypeString,
		Label:       "Model",
		Placeholder: "llama3.1:8b",
		Required:    true,
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
		// Tools handle. Open WebUI is OpenAI-compatible, so this uses
		// the OpenAI function-calling schema.
		Name:        "tool_definitions",
		Type:        core.ConnectionTypeText,
		Label:       "Tool Definitions (JSON)",
		Placeholder: `[{"type":"function","function":{"name":"web_search","description":"Search the web","parameters":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}}]`,
	},
	{
		Name:  "streaming",
		Type:  core.ConnectionTypeBoolean,
		Label: "Streaming (fires response per sentence for low-latency voice)",
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

// chatCompletionsURL normalises a user-supplied endpoint into a full Chat
// Completions URL. Callers may paste either a base URL
// (https://host) or a full endpoint already ending in /chat/completions;
// both are accepted so the action works against Open WebUI, LiteLLM, vLLM,
// Ollama's OpenAI shim, etc.
func chatCompletionsURL(endpoint string) string {
	e := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if strings.HasSuffix(e, "/chat/completions") {
		return e
	}
	return e + chatPath
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	endpointConn := core.FindConnection("endpoint", inputs)
	if endpointConn == nil || endpointConn.String() == nil || *endpointConn.String() == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	apiURL := chatCompletionsURL(*endpointConn.String())

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

	modelConn := core.FindConnection("model", inputs)
	if modelConn == nil || modelConn.String() == nil || *modelConn.String() == "" {
		return nil, fmt.Errorf("model is required")
	}
	model := *modelConn.String()

	// max_tokens and temperature are only sent when explicitly provided.
	// Open WebUI fronts arbitrary local/hosted models, some of which reject
	// unknown or unsupported sampling parameters, so we avoid forcing
	// defaults onto the request.
	var maxTokens *int64
	maxTokensConn := core.FindConnection("max_tokens", inputs)
	if maxTokensConn != nil && maxTokensConn.Number() != nil && *maxTokensConn.Number() > 0 {
		v := *maxTokensConn.Number()
		maxTokens = &v
	}

	var temperature *float64
	tempConn := core.FindConnection("temperature", inputs)
	if tempConn != nil && tempConn.String() != nil && *tempConn.String() != "" {
		var t float64
		if _, err := fmt.Sscanf(*tempConn.String(), "%f", &t); err == nil {
			temperature = &t
		}
	}

	systemPromptStr := ""
	systemConn := core.FindConnection("system_prompt", inputs)
	if systemConn != nil && systemConn.String() != nil && *systemConn.String() != "" {
		systemPromptStr = *systemConn.String()
	}

	// Parse tool definitions if provided (OpenAI-compatible format)
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
				budgetMaxTokens := defaultMaxTokens
				if maxTokens != nil {
					budgetMaxTokens = int(*maxTokens)
				}
				history = ai_common.TruncateHistoryForBudget(
					history, systemPromptStr, prompt,
					budgetMaxTokens, ai_common.ModelContextWindow(model),
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

		userContent := ai_common.BuildOpenAIUserContent(prompt, flow.Blobs())
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": userContent,
		})
	}

	// Streaming: fire the Response handle per sentence for low-latency
	// voice. Skipped on tool re-invocations — the tool loop needs the full
	// response to detect tool calls before deciding whether to stream.
	streaming := false
	if s := core.FindConnection("streaming", inputs); s != nil && s.String() != nil && *s.String() == "true" {
		streaming = true
	}
	isToolReinvocation := false
	if _, hasResults := flow.GetVariable(core.ToolResultsKey); hasResults {
		isToolReinvocation = true
	}
	useStreaming := streaming && !isToolReinvocation

	payload := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}
	if maxTokens != nil {
		payload["max_tokens"] = *maxTokens
	}
	if temperature != nil {
		payload["temperature"] = *temperature
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	if useStreaming {
		payload["stream"] = true
		payload["stream_options"] = map[string]interface{}{"include_usage": true}
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
		return nil, fmt.Errorf("Open WebUI request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
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
		return nil, fmt.Errorf("Open WebUI API error (%d): %s", resp.StatusCode, errMsg)
	}

	// Streaming response: the shared OpenAI-compatible SSE parser owns
	// resp.Body (closes it when the stream ends) and emits sentences via the
	// engine's streaming channel contract.
	if useStreaming {
		return ai_common.HandleOpenAICompatibleStream(flow, resp, model, map[string]interface{}{
			"prompt_tokens":     int64(0),
			"completion_tokens": int64(0),
			"total_tokens":      int64(0),
		})
	}

	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))

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
		return nil, fmt.Errorf("failed to parse Open WebUI response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("Open WebUI returned no choices")
	}

	// Some OpenAI-compatible backends echo the requested model rather than a
	// canonical id; fall back to the requested model when absent.
	modelUsed := result.Model
	if modelUsed == "" {
		modelUsed = model
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
			"model":                       modelUsed,
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
		// Record any accumulated tool exchanges before the final reply
		// so the conversation history includes what tools were called.
		if exchanges := extractToolExchanges(flow); len(exchanges) > 0 {
			ai_common.RecordToolExchange(flow.GoContext(), flow.GetContext(), exchanges)
			toolCallsCount = len(exchanges)
		}
		ai_common.RecordAssistantReply(flow.GoContext(), flow.GetContext(), content)
	}

	return map[string]interface{}{
		"response":          content,
		"should_respond":    shouldRespond,
		"model":             modelUsed,
		"prompt_tokens":     result.Usage.PromptTokens,
		"completion_tokens": result.Usage.CompletionTokens,
		"total_tokens":      result.Usage.TotalTokens,
		"tool_calls_count":  toolCallsCount,
		"success":           true,
		"error":             "",
	}, nil
}

// extractToolExchanges reads the accumulated tool exchanges from the
// flow variable set by the engine's tool loop. Returns nil if no tools
// were called in this turn.
func extractToolExchanges(flow *core.Flow) []ai_common.ToolExchange {
	raw, ok := flow.GetVariable(core.ToolExchangesKey)
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]map[string]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}
	exchanges := make([]ai_common.ToolExchange, 0, len(arr))
	for _, m := range arr {
		ex := ai_common.ToolExchange{}
		if v, ok := m["tool_use_id"].(string); ok {
			ex.ToolUseID = v
		}
		if v, ok := m["name"].(string); ok {
			ex.Name = v
		}
		if v, ok := m["input"].(map[string]interface{}); ok {
			ex.Input = v
		}
		if v, ok := m["result"].(string); ok {
			ex.Result = v
		}
		if v, ok := m["is_error"].(bool); ok {
			ex.IsError = v
		}
		exchanges = append(exchanges, ex)
	}
	return exchanges
}
