package anthropic

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
	log "github.com/sirupsen/logrus"
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
	defaultMaxTokens = 8192
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
	{
		Name:        "conversation_history",
		Type:        core.ConnectionTypeObject,
		Label:       "Conversation History",
		Placeholder: "${conversation_history}",
	},
	{
		// TEMPORARY: tool definitions as JSON. Will be replaced by
		// automatic discovery from the tools subgraph wired to the
		// Tools handle. For now, flow authors paste a JSON array of
		// Anthropic tool schemas here.
		Name:        "tool_definitions",
		Type:        core.ConnectionTypeText,
		Label:       "Tool Definitions (JSON)",
		Placeholder: `[{"name":"web_search","description":"Search the web","input_schema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}]`,
	},
}

var Outputs = [...]core.Connection{
	{Name: "response", Type: core.ConnectionTypeString, Label: "Response"},
	{Name: "response_mode", Type: core.ConnectionTypeString, Label: "Response Mode (text or voice)"},
	{Name: "should_respond", Type: core.ConnectionTypeBoolean, Label: "Should Respond"},
	{Name: "model", Type: core.ConnectionTypeString, Label: "Model Used"},
	{Name: "input_tokens", Type: core.ConnectionTypeInteger, Label: "Input Tokens"},
	{Name: "output_tokens", Type: core.ConnectionTypeInteger, Label: "Output Tokens"},
	{Name: "stop_reason", Type: core.ConnectionTypeString, Label: "Stop Reason"},
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

	// Parse tool definitions if provided
	var tools []interface{}
	toolDefsConn := core.FindConnection("tool_definitions", inputs)
	if toolDefsConn != nil {
		var raw string
		if s := toolDefsConn.String(); s != nil && *s != "" {
			raw = *s
		}
		if raw != "" {
			// Clean up common formatting issues — the editor's textarea
			// may store the JSON with newlines, tabs, or markdown fences
			raw = strings.ReplaceAll(raw, "\n", " ")
			raw = strings.ReplaceAll(raw, "\r", " ")
			raw = strings.ReplaceAll(raw, "\t", " ")
			raw = strings.TrimSpace(raw)
			// Strip markdown code fences if present
			if strings.HasPrefix(raw, "```") {
				if idx := strings.Index(raw[3:], "\n"); idx != -1 {
					raw = raw[3+idx+1:]
				}
				raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
				raw = strings.TrimSpace(raw)
			}
			if err := json.Unmarshal([]byte(raw), &tools); err != nil {
				log.WithFields(log.Fields{
					"error": err,
					"raw":   raw[:min(len(raw), 100)],
				}).Warn("failed to parse tool_definitions JSON")
			} else {
				log.WithFields(log.Fields{
					"count": len(tools),
				}).Info("parsed tool definitions")
			}
		}
	}

	// Check if we're in a tool loop (re-invocation with tool results).
	// The engine sets __tool_results and __conversation_state as flow
	// variables before re-invoking this action.
	var messages []interface{}
	toolCallsCount := 0

	if convState, ok := flow.GetVariable(core.ToolConversationStateKey); ok && convState != nil {
		// Continuing a tool loop — restore the conversation state
		if ms, ok := convState.([]interface{}); ok {
			messages = ms
		}

		// Append tool results as tool_result content blocks.
		// Estimate token budget: if conversation is getting large, truncate
		// individual tool results to prevent exhausting the context window.
		if toolResults, ok := flow.GetVariable(core.ToolResultsKey); ok && toolResults != nil {
			if results, ok := toolResults.([]core.ToolResult); ok {
				// Estimate current message tokens to decide truncation
				contextWindow := ai_common.ModelContextWindow(model)
				currentTokens := estimateMessagesTokens(messages) +
					ai_common.ApproxTokens(systemPromptStr) + int(maxTokens) + 64
				remainingBudget := contextWindow - currentTokens
				// Reserve at least 30% of context for tool results + response
				maxResultTokens := remainingBudget
				if maxResultTokens < 0 {
					maxResultTokens = 4096 // fallback minimum
				}
				// Per-result limit: divide remaining budget among results
				perResultLimit := maxResultTokens / max(len(results), 1)
				// Minimum 512 tokens per result, max 8192
				if perResultLimit < 512 {
					perResultLimit = 512
				}
				if perResultLimit > 8192 {
					perResultLimit = 8192
				}
				perResultChars := perResultLimit * 4 // ~4 chars per token

				for _, r := range results {
					resultContent := r.Content
					if len(resultContent) > perResultChars {
						resultContent = resultContent[:perResultChars] + "\n... [truncated — full result too large for context window]"
						log.WithFields(log.Fields{
							"tool_use_id":    r.ToolUseID,
							"original_chars": len(r.Content),
							"truncated_to":   perResultChars,
						}).Info("truncated tool result to fit context budget")
					}
					content := []map[string]interface{}{
						{
							"type":        "tool_result",
							"tool_use_id": r.ToolUseID,
							"content":     resultContent,
						},
					}
					if r.IsError {
						content[0]["is_error"] = true
					}
					messages = append(messages, map[string]interface{}{
						"role":    "user",
						"content": content,
					})
				}
			}
		}
	} else {
		// First invocation — build messages from history + prompt
		historyConn := core.FindConnection("conversation_history", inputs)
		if historyConn != nil {
			history := ai_common.ParseConversationHistory(historyConn.Value)
			if len(history) > 0 {
				history = ai_common.TruncateHistoryForBudget(
					history, systemPromptStr, prompt,
					int(maxTokens), ai_common.ModelContextWindow(model),
				)
				for i, m := range history {
					if m.Role == "" || m.Content == "" {
						continue
					}
					if m.Role != "user" && m.Role != "assistant" {
						continue
					}
					msg := map[string]interface{}{
						"role": m.Role,
					}
					// Mark the last history message with cache_control
					// so the entire history prefix is cached. The new
					// user prompt (appended below) is the only uncached part.
					if i == len(history)-1 {
						msg["content"] = []map[string]interface{}{
							{
								"type":          "text",
								"text":          m.Content,
								"cache_control": map[string]string{"type": "ephemeral"},
							},
						}
					} else {
						msg["content"] = m.Content
					}
					messages = append(messages, msg)
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
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"messages":    messages,
	}

	// System prompt: use structured content blocks with cache_control
	// so repeated turns with the same system prompt hit the cache.
	if systemPromptStr != "" {
		payload["system"] = []map[string]interface{}{
			{
				"type":          "text",
				"text":          systemPromptStr,
				"cache_control": map[string]string{"type": "ephemeral"},
			},
		}
	}

	// Tools: mark the last tool with cache_control so the entire tool
	// definition block is cached across turns.
	if len(tools) > 0 {
		// Add cache_control to the last tool definition.
		if toolMap, ok := tools[len(tools)-1].(map[string]interface{}); ok {
			toolMap["cache_control"] = map[string]string{"type": "ephemeral"}
		}
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
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Anthropic request failed: %w", err)
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
		return nil, fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, errMsg)
	}

	var result struct {
		Content []struct {
			Type  string                 `json:"type"`
			Text  string                 `json:"text,omitempty"`
			ID    string                 `json:"id,omitempty"`
			Name  string                 `json:"name,omitempty"`
			Input map[string]interface{} `json:"input,omitempty"`
		} `json:"content"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	// Check for tool_use blocks in the response
	if result.StopReason == "tool_use" {
		var toolRequests []core.ToolRequest
		var assistantContent []interface{}
		var intermediateText string

		for _, block := range result.Content {
			switch block.Type {
			case "tool_use":
				toolRequests = append(toolRequests, core.ToolRequest{
					ID:    block.ID,
					Name:  block.Name,
					Input: block.Input,
				})
				assistantContent = append(assistantContent, map[string]interface{}{
					"type":  "tool_use",
					"id":    block.ID,
					"name":  block.Name,
					"input": block.Input,
				})
			case "text":
				if block.Text != "" {
					assistantContent = append(assistantContent, map[string]interface{}{
						"type": "text",
						"text": block.Text,
					})
					// Capture intermediate text so the engine can send it
					// to the user via the Response handle mid-tool-loop.
					if intermediateText != "" {
						intermediateText += "\n"
					}
					intermediateText += block.Text
				}
			}
		}

		// Append the assistant's tool_use message to the conversation
		messages = append(messages, map[string]interface{}{
			"role":    "assistant",
			"content": assistantContent,
		})

		out := map[string]interface{}{
			core.ToolRequestsKey:         toolRequests,
			core.ToolConversationStateKey: messages,
			"response_mode":              "text", // default during tool calls; final response may override
			"stop_reason":                result.StopReason,
			"model":                      result.Model,
			"input_tokens":               result.Usage.InputTokens,
			"output_tokens":              result.Usage.OutputTokens,
			"tool_calls_count":           len(toolRequests),
			"success":                    true,
			"error":                      "",
		}
		if intermediateText != "" {
			out[core.IntermediateTextKey] = intermediateText
		}
		return out, nil
	}

	// Final text response — extract content and record for agent memory
	var content string
	for _, block := range result.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	// Detect max_tokens truncation with an empty response — this means
	// the conversation history consumed the entire context window and the
	// model had no room to generate a response. Fail the node so the
	// error is visible rather than silently sending an empty message.
	if result.StopReason == "max_tokens" && strings.TrimSpace(content) == "" {
		log.WithFields(log.Fields{
			"model":         result.Model,
			"input_tokens":  result.Usage.InputTokens,
			"output_tokens": result.Usage.OutputTokens,
			"max_tokens":    maxTokens,
		}).Error("AI response truncated to empty — conversation history too large for context window")

		return map[string]interface{}{
			"response":         "",
			"should_respond":   false,
			"model":            result.Model,
			"input_tokens":     result.Usage.InputTokens,
			"output_tokens":    result.Usage.OutputTokens,
			"stop_reason":      result.StopReason,
			"tool_calls_count": toolCallsCount,
			"success":          false,
			"error":            fmt.Sprintf("Response truncated — conversation history (%d input tokens) exhausted the context window, leaving no room for a response. Try reducing conversation history or tool call count.", result.Usage.InputTokens),
		}, nil
	}

	// Check if the AI decided no response is needed (e.g. message not
	// directed at this agent in a multi-user channel). The AI signals
	// this by including [NO_RESPONSE] in its output.
	shouldRespond := true
	trimmed := strings.TrimSpace(content)
	if trimmed == "[NO_RESPONSE]" || strings.Contains(trimmed, "[NO_RESPONSE]") {
		shouldRespond = false
		content = "" // Don't record or send empty responses
	}

	// Parse response mode prefix: [VOICE] or [TEXT]. The AI includes
	// this when the channel supports multiple response formats (e.g.
	// telegram_voice conversations where the agent can reply with
	// either a voice note or a text message). Default is "text".
	// Voice mode is only permitted when the inbound channel is
	// telegram_voice — text channels must always respond with text.
	responseMode := "text"
	contentTrimmed := strings.TrimSpace(content)
	if strings.HasPrefix(contentTrimmed, "[VOICE]") {
		content = strings.TrimSpace(strings.TrimPrefix(contentTrimmed, "[VOICE]"))
		// Only allow voice responses on voice-capable channels
		ctx := flow.GetContext()
		if ctx != nil && ctx.ChannelType == "telegram_voice" {
			responseMode = "voice"
		}
	} else if strings.HasPrefix(contentTrimmed, "[TEXT]") {
		responseMode = "text"
		content = strings.TrimSpace(strings.TrimPrefix(contentTrimmed, "[TEXT]"))
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
		"response":         content,
		"response_mode":    responseMode,
		"should_respond":   shouldRespond,
		"model":            result.Model,
		"input_tokens":     result.Usage.InputTokens,
		"output_tokens":    result.Usage.OutputTokens,
		"stop_reason":      result.StopReason,
		"tool_calls_count": toolCallsCount,
		"success":          true,
		"error":            "",
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

// estimateMessagesTokens provides a rough token count for the messages
// array used in the Anthropic API call. This is an approximation used
// for budget-aware tool result truncation — not a precise tokeniser.
func estimateMessagesTokens(messages []interface{}) int {
	b, err := json.Marshal(messages)
	if err != nil {
		return 0
	}
	return ai_common.ApproxTokens(string(b))
}
