// Package openrouter implements the ai/openrouter action: a chat-completion
// call against OpenRouter's unified, OpenAI-compatible Chat Completions API.
// OpenRouter fronts hundreds of models from every major provider behind one
// endpoint and one API key, with model ids namespaced by provider
// ("anthropic/claude-sonnet-5", "openai/gpt-5.4", ...).
//
// Because the wire format mirrors OpenAI exactly, this package deliberately
// tracks actions/ai/groq (which in turn tracks actions/ai/openai) — including
// the tool-calling loop, vision image-block promotion, conversation-history
// truncation, and the [NO_RESPONSE] sentinel. On top of that shared shape it
// adds the sampling controls OpenRouter forwards to every provider
// (top_p, frequency/presence penalties, JSON response format) and surfaces
// OpenRouter's per-request cost and winning provider in the outputs.
package openrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	ai_common "flomation.app/automate/executor/actions/ai"

	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OpenRouter Prompt"
	Description  = "Send a prompt to any AI model through the OpenRouter unified API and return the response"
	Website      = "https://www.flomation.co"
	Icon         = "brain+route"
	Date         = "02/07/2026"
	Type         = core.ActionTypeAction

	// OpenRouter exposes an OpenAI-compatible Chat Completions endpoint, so
	// the request/response shapes mirror actions/ai/openai exactly. The
	// provider-specific bits are the base URL, the model catalogue, the
	// optional attribution headers, and the cost/provider response fields.
	defaultModel     = "openai/gpt-5.4-mini"
	defaultMaxTokens = 2048
	maxResponseBody  = 1 << 20 // 1 MB

	// OpenRouter's recommended app-attribution headers. They are optional,
	// never affect routing or billing, and identify Flomation on
	// openrouter.ai's public app rankings.
	attributionReferer = "https://www.flomation.co"
	attributionTitle   = "Flomation"
)

// apiURL is the OpenRouter Chat Completions endpoint. It is a var rather
// than a const so tests can point it at an httptest server.
var apiURL = "https://openrouter.ai/api/v1/chat/completions"

// httpClient is shared across calls so connections to the OpenRouter
// endpoint are pooled and reused rather than re-dialled on every invocation.
// The timeout is longer than other AI actions' 120s because OpenRouter
// routes to reasoning models that can legitimately think for minutes.
var httpClient = &http.Client{Timeout: 360 * time.Second}

var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Key",
		Placeholder: "sk-or-v1-...",
		Required:    true,
	},
	{
		// Free-text (string) rather than a picker-only field so users can
		// type any of OpenRouter's 300+ current or future model ids, while
		// the Options below surface flagship choices as suggestions.
		// OpenRouter's catalogue changes daily, so locking the field to a
		// fixed list would strand new models.
		Name:        "model",
		Type:        core.ConnectionTypeString,
		Label:       "Model",
		Placeholder: "openai/gpt-5.4-mini",
		// Curated from OpenRouter's live catalogue
		// (GET https://openrouter.ai/api/v1/models). Verified live on
		// 02/07/2026 — every entry returns a successful completion.
		Options: []core.ConnectionOption{
			{Name: "OpenAI GPT-5.4", Value: "openai/gpt-5.4"},
			{Name: "OpenAI GPT-5.4 Mini", Value: "openai/gpt-5.4-mini"},
			{Name: "Anthropic Claude Sonnet 5", Value: "anthropic/claude-sonnet-5"},
			{Name: "Anthropic Claude Opus 4.8", Value: "anthropic/claude-opus-4.8"},
			{Name: "Anthropic Claude Haiku 4.5", Value: "anthropic/claude-haiku-4.5"},
			{Name: "Google Gemini 3.1 Pro Preview", Value: "google/gemini-3.1-pro-preview"},
			{Name: "Google Gemini 2.5 Flash", Value: "google/gemini-2.5-flash"},
			{Name: "Meta Llama 4 Maverick", Value: "meta-llama/llama-4-maverick"},
			{Name: "DeepSeek V3.2", Value: "deepseek/deepseek-v3.2"},
			{Name: "Qwen3 Max", Value: "qwen/qwen3-max"},
			{Name: "Mistral Large 2512", Value: "mistralai/mistral-large-2512"},
			{Name: "GPT-OSS 20B (Free)", Value: "openai/gpt-oss-20b:free"},
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
		// The sampling controls below are strings because the platform has
		// no float connection type (only Integer). Each is omitted from the
		// request entirely when left blank, so provider defaults apply —
		// some models reject sampling params they don't support.
		Name:        "top_p",
		Type:        core.ConnectionTypeString,
		Label:       "Top P",
		Placeholder: "1.0",
	},
	{
		Name:        "frequency_penalty",
		Type:        core.ConnectionTypeString,
		Label:       "Frequency Penalty",
		Placeholder: "0.0 (-2.0 to 2.0)",
	},
	{
		Name:        "presence_penalty",
		Type:        core.ConnectionTypeString,
		Label:       "Presence Penalty",
		Placeholder: "0.0 (-2.0 to 2.0)",
	},
	{
		// JSON mode instructs the model to emit valid JSON. Like OpenAI's
		// implementation, the word "json" should appear in the prompt or
		// system prompt for models to reliably comply.
		Name:  "response_format",
		Type:  core.ConnectionTypeString,
		Label: "Response Format",
		Options: []core.ConnectionOption{
			{Name: "Text", Value: "text"},
			{Name: "JSON", Value: "json_object"},
		},
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
	{
		Name:  "streaming",
		Type:  core.ConnectionTypeBoolean,
		Label: "Streaming (fires response per sentence for low-latency voice)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "response", Type: core.ConnectionTypeString, Label: "Response"},
	{Name: "model", Type: core.ConnectionTypeString, Label: "Model Used"},
	{Name: "provider", Type: core.ConnectionTypeString, Label: "Provider"},
	{Name: "prompt_tokens", Type: core.ConnectionTypeInteger, Label: "Prompt Tokens"},
	{Name: "completion_tokens", Type: core.ConnectionTypeInteger, Label: "Completion Tokens"},
	{Name: "total_tokens", Type: core.ConnectionTypeInteger, Label: "Total Tokens"},
	{Name: "cost", Type: core.ConnectionTypeString, Label: "Cost (USD)"},
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

	// Temperature is a string input because the platform has no float
	// connection type (only Integer); see the action's Inputs. Parse it
	// explicitly so malformed values warn loudly and fall back to the
	// default rather than being silently swallowed.
	// Temperature is opt-in — OpenRouter proxies to any provider (incl. models
	// that reject a non-default temperature), so only send a value the author
	// explicitly set rather than a forced default.
	var temperature *float64
	tempConn := core.FindConnection("temperature", inputs)
	if tempConn != nil && tempConn.String() != nil && *tempConn.String() != "" {
		raw := strings.TrimSpace(*tempConn.String())
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			temperature = &parsed
		} else {
			log.WithFields(log.Fields{
				"value": raw,
			}).Warn("[openrouter] invalid temperature; omitting it")
		}
	}

	// Optional sampling controls — only sent when the user set a valid
	// value, so provider defaults apply otherwise.
	topP := optionalFloat("top_p", inputs)
	frequencyPenalty := optionalFloat("frequency_penalty", inputs)
	presencePenalty := optionalFloat("presence_penalty", inputs)

	// The editor renders this as a strict dropdown, but the value can also
	// arrive via ${...} substitution or API-created flows, so normalise and
	// warn on anything unrecognised instead of silently sending text mode.
	responseFormat := ""
	if rfConn := core.FindConnection("response_format", inputs); rfConn != nil && rfConn.String() != nil {
		responseFormat = strings.ToLower(strings.TrimSpace(*rfConn.String()))
	}
	switch responseFormat {
	case "", "text", "json_object":
	case "json":
		responseFormat = "json_object"
	default:
		log.WithFields(log.Fields{
			"value":   responseFormat,
			"default": "text",
		}).Warn("[openrouter] invalid response_format; falling back to text")
		responseFormat = ""
	}

	systemPromptStr := ""
	systemConn := core.FindConnection("system_prompt", inputs)
	if systemConn != nil && systemConn.String() != nil && *systemConn.String() != "" {
		systemPromptStr = *systemConn.String()
	}
	// Teach the model to pass flo:blob:<handle> tokens verbatim to
	// downstream tools rather than inventing placeholder strings
	// for large outputs it can't see. See ai_common for the
	// rationale.
	systemPromptStr = ai_common.AppendBlobTokenInstructions(systemPromptStr)

	// Parse tool definitions if provided (OpenAI format — OpenRouter is
	// OpenAI-compatible and accepts the same tools/tool_calls schema).
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

		// Vision-block promotion: if the prompt carries [attached: ...]
		// markers for image attachments, resolve their blob bytes and
		// upgrade the user content from a plain string to a content-
		// block array using OpenAI's image_url / data: URL shape, which
		// OpenRouter forwards verbatim to vision-capable models.
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
		"model":      model,
		"messages":   messages,
		"max_tokens": maxTokens,
	}
	if temperature != nil {
		payload["temperature"] = *temperature
	}
	if topP != nil {
		payload["top_p"] = *topP
	}
	if frequencyPenalty != nil {
		payload["frequency_penalty"] = *frequencyPenalty
	}
	if presencePenalty != nil {
		payload["presence_penalty"] = *presencePenalty
	}
	if responseFormat == "json_object" {
		payload["response_format"] = map[string]interface{}{"type": "json_object"}
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	if useStreaming {
		payload["stream"] = true
		// Ask for a terminal usage chunk so token counts survive the stream.
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
	req.Header.Set("HTTP-Referer", attributionReferer)
	req.Header.Set("X-Title", attributionTitle)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		// Best-effort decode of the structured error; on failure we fall
		// back to the raw body below, so the unmarshal error is ignored
		// deliberately rather than swallowed.
		_ = json.Unmarshal(respBody, &apiErr)
		errMsg := apiErr.Error.Message
		if errMsg == "" {
			errMsg = string(respBody)
		}
		return nil, fmt.Errorf("OpenRouter API error (%d): %s", resp.StatusCode, errMsg)
	}

	// Streaming response: the shared OpenAI-compatible SSE parser owns
	// resp.Body (closes it when the stream ends) and emits sentences via the
	// engine's streaming channel contract. OpenRouter's extra provider/cost
	// outputs are seeded empty so downstream references don't get nil.
	if useStreaming {
		return ai_common.HandleOpenAICompatibleStream(flow, resp, model, map[string]interface{}{
			"prompt_tokens":     int64(0),
			"completion_tokens": int64(0),
			"total_tokens":      int64(0),
			"provider":          "",
			"cost":              "",
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
		Model    string `json:"model"`
		Provider string `json:"provider"`
		Usage    struct {
			PromptTokens     int64   `json:"prompt_tokens"`
			CompletionTokens int64   `json:"completion_tokens"`
			TotalTokens      int64   `json:"total_tokens"`
			Cost             float64 `json:"cost"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse OpenRouter response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("OpenRouter returned no choices")
	}

	choice := result.Choices[0]
	cost := strconv.FormatFloat(result.Usage.Cost, 'f', -1, 64)

	// Check for tool calls
	if choice.FinishReason == "tool_calls" && len(choice.Message.ToolCalls) > 0 {
		var toolRequests []core.ToolRequest

		// Some providers behind OpenRouter (historically Anthropic) return
		// empty-string arguments for tools that take no parameters, instead
		// of the "{}" the OpenAI schema specifies. Normalise before the
		// arguments are parsed or echoed back in the conversation state —
		// providers reject "" on the follow-up request.
		for i := range choice.Message.ToolCalls {
			if strings.TrimSpace(choice.Message.ToolCalls[i].Function.Arguments) == "" {
				choice.Message.ToolCalls[i].Function.Arguments = "{}"
			}
		}

		// Build the assistant message with tool_calls for conversation state
		assistantMsg := map[string]interface{}{
			"role":       "assistant",
			"tool_calls": choice.Message.ToolCalls,
		}
		if choice.Message.Content != nil {
			assistantMsg["content"] = *choice.Message.Content
		}

		for _, tc := range choice.Message.ToolCalls {
			input := map[string]interface{}{}
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
			"provider":                    result.Provider,
			"prompt_tokens":               result.Usage.PromptTokens,
			"completion_tokens":           result.Usage.CompletionTokens,
			"total_tokens":                result.Usage.TotalTokens,
			"cost":                        cost,
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
	if ai_common.SuppressResponse(content) {
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
		"model":             result.Model,
		"provider":          result.Provider,
		"prompt_tokens":     result.Usage.PromptTokens,
		"completion_tokens": result.Usage.CompletionTokens,
		"total_tokens":      result.Usage.TotalTokens,
		"cost":              cost,
		"tool_calls_count":  toolCallsCount,
		"success":           true,
		"error":             "",
	}, nil
}

// optionalFloat parses an optional string-typed numeric input. Returns nil
// when the input is absent or blank; malformed values warn and are omitted
// from the request so provider defaults apply.
func optionalFloat(name string, inputs []*core.Connection) *float64 {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return nil
	}
	raw := strings.TrimSpace(*conn.String())
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		log.WithFields(log.Fields{
			"input": name,
			"value": raw,
		}).Warn("[openrouter] invalid number; omitting from request")
		return nil
	}
	return &parsed
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
