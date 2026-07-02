// Package ollama implements the ai/ollama action: a chat call against a
// self-hosted Ollama server's native /api/chat endpoint.
//
// This deliberately speaks Ollama's native protocol rather than its OpenAI
// shim (/v1/chat/completions) because the native API is the only way to
// reach the features that make local models worth running: thinking mode
// (think), model keep-alive, JSON/structured output via the top-level
// format field, the full llama.cpp sampling/runtime option set (num_ctx,
// repeat_penalty, mirostat-era knobs), native prompt/eval token counts,
// and vision via base64 image arrays on messages.
//
// The message-assembly skeleton (history truncation, tool loop via engine
// flow variables, [NO_RESPONSE] sentinel, conversation recording) tracks
// actions/ai/openrouter. The wire differences from the OpenAI family:
//
//   - Tool calls carry no IDs and their arguments are JSON objects, not
//     strings. IDs are synthesised here so the engine's ToolRequest/
//     ToolResult round-trip works, and results are echoed back as
//     {role: "tool", tool_name: ...} messages (Ollama matches results to
//     calls by name, not id).
//   - Sampling options nest under "options"; format/keep_alive/think are
//     top-level. (n8n's Ollama node sends format and keep_alive inside
//     options where the server silently ignores them — not replicated.)
//   - stream defaults to TRUE on /api/chat, so stream:false is explicit.
package ollama

import (
	"bytes"
	"encoding/base64"
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
	Name         = "Ollama Prompt"
	Description  = "Send a prompt to a model running on your own Ollama server and return the response"
	Website      = "https://www.flomation.co"
	Icon         = "brain+server"
	Date         = "02/07/2026"
	Type         = core.ActionTypeAction

	chatPath         = "/api/chat"
	defaultMaxTokens = 2048
	maxResponseBody  = 1 << 20 // 1 MB
)

// httpClient is shared across calls so connections are pooled. The timeout
// matches openrouter's 360s rather than the 120s of other AI actions:
// a local server may spend minutes cold-loading a model from disk before
// the first token, and reasoning models think on top of that.
var httpClient = &http.Client{Timeout: 360 * time.Second}

var Inputs = [...]core.Connection{
	{
		Name:        "endpoint",
		Type:        core.ConnectionTypeString,
		Label:       "Ollama Server URL",
		Placeholder: "http://localhost:11434",
		Required:    true,
	},
	{
		// Optional: a default Ollama install is unauthenticated. Only
		// needed when the server sits behind an authenticating reverse
		// proxy, in which case the value is sent as a Bearer token.
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Key",
		Placeholder: "Only needed behind an authenticated proxy",
	},
	{
		// Free text with suggestions rather than a locked dropdown: the
		// real model list is whatever the user has pulled onto their own
		// server. The api layer overlays this input with a live dropdown
		// resolved from the server's /api/tags at edit time; these static
		// Options are the editor's fallback when that fetch fails.
		Name:        "model",
		Type:        core.ConnectionTypeString,
		Label:       "Model",
		Placeholder: "llama3.2",
		Required:    true,
		Options: []core.ConnectionOption{
			{Name: "Llama 3.2", Value: "llama3.2"},
			{Name: "Llama 3.1 8B", Value: "llama3.1:8b"},
			{Name: "Qwen 3", Value: "qwen3"},
			{Name: "Gemma 3", Value: "gemma3"},
			{Name: "Mistral", Value: "mistral"},
			{Name: "Phi-4", Value: "phi4"},
			{Name: "DeepSeek-R1", Value: "deepseek-r1"},
			{Name: "Qwen 2.5 Coder", Value: "qwen2.5-coder"},
			{Name: "Llama 3.2 Vision", Value: "llama3.2-vision"},
			{Name: "LLaVA (Vision)", Value: "llava"},
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
		Name:        "conversation_history",
		Type:        core.ConnectionTypeObject,
		Label:       "Conversation History",
		Placeholder: "${conversation_history}",
	},
	{
		// Maps to Ollama's options.num_predict. -1 means unlimited.
		Name:        "max_tokens",
		Type:        core.ConnectionTypeInteger,
		Label:       "Max Tokens",
		Placeholder: "1024 (-1 = unlimited)",
	},
	{
		// Strings because the platform has no float connection type (only
		// Integer). Every sampling control below is omitted from the
		// request when blank so the model's own defaults apply.
		Name:        "temperature",
		Type:        core.ConnectionTypeString,
		Label:       "Temperature",
		Placeholder: "0.8",
	},
	{
		// Maps to the native top-level format field. "json" constrains
		// output to valid JSON; as with the other AI actions, the word
		// "json" should appear in the prompt for reliable compliance.
		// A raw JSON-schema object (arriving via ${...} substitution or
		// API-authored flows) is forwarded for structured outputs.
		Name:  "response_format",
		Type:  core.ConnectionTypeString,
		Label: "Response Format",
		Options: []core.ConnectionOption{
			{Name: "Text", Value: "text"},
			{Name: "JSON", Value: "json"},
		},
	},
	{
		// Top-level think field. Left unset, the model's default applies —
		// forcing it on errors on models without thinking support, and
		// forcing it off errors on models that cannot disable it, so a
		// deliberate tri-state rather than a boolean. The effort levels
		// are the string form of think that reasoning-effort models
		// (gpt-oss family) accept in place of true.
		Name:  "think",
		Type:  core.ConnectionTypeString,
		Label: "Thinking",
		Options: []core.ConnectionOption{
			{Name: "Model Default", Value: "default"},
			{Name: "On", Value: "on"},
			{Name: "Off", Value: "off"},
			{Name: "On (Low Effort)", Value: "low"},
			{Name: "On (Medium Effort)", Value: "medium"},
			{Name: "On (High Effort)", Value: "high"},
		},
	},
	{
		// Gates the llama.cpp tuning knobs below so the default form stays
		// approachable; visibility only — the runtime reads the gated
		// inputs regardless, so values set via the API always apply.
		Name:  "advanced",
		Type:  core.ConnectionTypeString,
		Label: "Advanced Options",
		Options: []core.ConnectionOption{
			{Name: "Hide", Value: "hide"},
			{Name: "Show", Value: "show"},
		},
	},
	{
		Name:        "top_p",
		Type:        core.ConnectionTypeString,
		Label:       "Top P",
		Placeholder: "0.9",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "top_k",
		Type:        core.ConnectionTypeInteger,
		Label:       "Top K",
		Placeholder: "40",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "min_p",
		Type:        core.ConnectionTypeString,
		Label:       "Min P",
		Placeholder: "0.0",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "seed",
		Type:        core.ConnectionTypeInteger,
		Label:       "Seed",
		Placeholder: "Set for reproducible output",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "stop",
		Type:        core.ConnectionTypeString,
		Label:       "Stop Sequences",
		Placeholder: "Comma-separated, e.g. ###,END",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		// Top-level keep_alive: how long the model stays loaded in memory
		// after the call ("5m", "1h30m", "-1" = forever, "0" = unload).
		Name:        "keep_alive",
		Type:        core.ConnectionTypeString,
		Label:       "Keep Alive",
		Placeholder: "5m",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "num_ctx",
		Type:        core.ConnectionTypeInteger,
		Label:       "Context Length",
		Placeholder: "4096",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "repeat_penalty",
		Type:        core.ConnectionTypeString,
		Label:       "Repetition Penalty",
		Placeholder: "1.1 (1.0 = off)",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "repeat_last_n",
		Type:        core.ConnectionTypeInteger,
		Label:       "Repeat Window",
		Placeholder: "64 (0 = off, -1 = context length)",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "frequency_penalty",
		Type:        core.ConnectionTypeString,
		Label:       "Frequency Penalty",
		Placeholder: "0.0",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "presence_penalty",
		Type:        core.ConnectionTypeString,
		Label:       "Presence Penalty",
		Placeholder: "0.0",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "num_batch",
		Type:        core.ConnectionTypeInteger,
		Label:       "Prompt Batch Size",
		Placeholder: "512",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		// Ollama's num_gpu is the number of model layers offloaded to the
		// GPU (not a GPU count).
		Name:        "num_gpu",
		Type:        core.ConnectionTypeInteger,
		Label:       "GPU Layers",
		Placeholder: "-1 (auto)",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "main_gpu",
		Type:        core.ConnectionTypeInteger,
		Label:       "Main GPU",
		Placeholder: "0",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:        "num_thread",
		Type:        core.ConnectionTypeInteger,
		Label:       "CPU Threads",
		Placeholder: "0 (auto)",
		Visible:     &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:    "low_vram",
		Type:    core.ConnectionTypeBoolean,
		Label:   "Low VRAM Mode",
		Visible: &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:    "use_mlock",
		Type:    core.ConnectionTypeBoolean,
		Label:   "Lock Model in RAM (mlock)",
		Visible: &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:    "use_mmap",
		Type:    core.ConnectionTypeBoolean,
		Label:   "Memory-Map Model (mmap)",
		Visible: &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		Name:    "penalize_newline",
		Type:    core.ConnectionTypeBoolean,
		Label:   "Penalize Newlines",
		Visible: &core.VisibleWhen{Field: "advanced", Values: []string{"show"}},
	},
	{
		// TEMPORARY: tool definitions as JSON, matching the other AI
		// actions. The engine overwrites this with auto-generated
		// definitions when nodes are wired to the Tools handle; both the
		// engine's Anthropic-style {name, input_schema} entries and
		// OpenAI-style {type: "function", function: {...}} entries are
		// accepted and normalised to Ollama's tool schema.
		Name:        "tool_definitions",
		Type:        core.ConnectionTypeText,
		Label:       "Tool Definitions (JSON)",
		Placeholder: `[{"type":"function","function":{"name":"web_search","description":"Search the web","parameters":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}}]`,
	},
}

var Outputs = [...]core.Connection{
	{Name: "response", Type: core.ConnectionTypeString, Label: "Response"},
	{Name: "thinking", Type: core.ConnectionTypeString, Label: "Thinking"},
	{Name: "model", Type: core.ConnectionTypeString, Label: "Model Used"},
	{Name: "prompt_tokens", Type: core.ConnectionTypeInteger, Label: "Prompt Tokens"},
	{Name: "completion_tokens", Type: core.ConnectionTypeInteger, Label: "Completion Tokens"},
	{Name: "total_tokens", Type: core.ConnectionTypeInteger, Label: "Total Tokens"},
	{Name: "should_respond", Type: core.ConnectionTypeBoolean, Label: "Should Respond"},
	{Name: "tool_calls_count", Type: core.ConnectionTypeInteger, Label: "Tool Calls"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// chatURL normalises a user-supplied endpoint into the native chat URL.
// Users paste the base URL (http://host:11434), but pasting the full
// /api/chat path or a bare /api suffix is accepted too.
func chatURL(endpoint string) string {
	e := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	e = strings.TrimSuffix(e, "/api/chat")
	e = strings.TrimSuffix(e, "/api")
	return e + chatPath
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	endpointConn := core.FindConnection("endpoint", inputs)
	if endpointConn == nil || endpointConn.String() == nil || *endpointConn.String() == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	apiURL := chatURL(*endpointConn.String())

	apiKey := ""
	if apiKeyConn := core.FindConnection("api_key", inputs); apiKeyConn != nil && apiKeyConn.String() != nil {
		apiKey = strings.TrimSpace(*apiKeyConn.String())
	}

	modelConn := core.FindConnection("model", inputs)
	if modelConn == nil || modelConn.String() == nil || *modelConn.String() == "" {
		return nil, fmt.Errorf("model is required")
	}
	model := *modelConn.String()

	promptConn := core.FindConnection("prompt", inputs)
	if promptConn == nil || promptConn.String() == nil || *promptConn.String() == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	prompt := *promptConn.String()

	// Every option below is omitted from the request when unset so the
	// model/server defaults apply — local models vary too much for the
	// action to impose its own.
	options := map[string]interface{}{}

	var maxTokens *int64
	if conn := core.FindConnection("max_tokens", inputs); conn != nil && conn.Number() != nil {
		if v := *conn.Number(); v > 0 || v == -1 {
			maxTokens = &v
			options["num_predict"] = v
		}
	}
	for name, key := range map[string]string{
		"temperature":       "temperature",
		"top_p":             "top_p",
		"min_p":             "min_p",
		"repeat_penalty":    "repeat_penalty",
		"frequency_penalty": "frequency_penalty",
		"presence_penalty":  "presence_penalty",
	} {
		if v := optionalFloat(name, inputs); v != nil {
			options[key] = *v
		}
	}
	var numCtx *int64
	for name, key := range map[string]string{
		"top_k":         "top_k",
		"seed":          "seed",
		"num_ctx":       "num_ctx",
		"repeat_last_n": "repeat_last_n",
		"num_batch":     "num_batch",
		"num_gpu":       "num_gpu",
		"main_gpu":      "main_gpu",
		"num_thread":    "num_thread",
	} {
		if conn := core.FindConnection(name, inputs); conn != nil && conn.Number() != nil {
			v := *conn.Number()
			options[key] = v
			if name == "num_ctx" {
				numCtx = &v
			}
		}
	}
	for name, key := range map[string]string{
		"low_vram":         "low_vram",
		"use_mlock":        "use_mlock",
		"use_mmap":         "use_mmap",
		"penalize_newline": "penalize_newline",
	} {
		if conn := core.FindConnection(name, inputs); conn != nil && conn.Boolean() != nil {
			options[key] = *conn.Boolean()
		}
	}
	if conn := core.FindConnection("stop", inputs); conn != nil && conn.String() != nil && *conn.String() != "" {
		var stops []string
		for _, s := range strings.Split(*conn.String(), ",") {
			if s = strings.TrimSpace(s); s != "" {
				stops = append(stops, s)
			}
		}
		if len(stops) > 0 {
			options["stop"] = stops
		}
	}

	// format is a top-level request field: "json" for JSON mode, or a
	// JSON-schema object for structured outputs when the value arrives as
	// raw JSON via ${...} substitution or API-authored flows.
	var format interface{}
	if conn := core.FindConnection("response_format", inputs); conn != nil && conn.String() != nil {
		raw := strings.TrimSpace(*conn.String())
		switch strings.ToLower(raw) {
		case "", "text", "default":
		case "json", "json_object":
			format = "json"
		default:
			var schema map[string]interface{}
			if strings.HasPrefix(raw, "{") && json.Unmarshal([]byte(raw), &schema) == nil {
				format = schema
			} else {
				log.WithFields(log.Fields{
					"value":   raw,
					"default": "text",
				}).Warn("[ollama] invalid response_format; falling back to text")
			}
		}
	}

	// think is top-level and deliberately tri-state; sending it at all to
	// a model that can't honour it is a server-side error. The field is a
	// bool-or-string union upstream: reasoning-effort models take
	// "low"/"medium"/"high" in place of true.
	var think interface{}
	if conn := core.FindConnection("think", inputs); conn != nil && conn.String() != nil {
		switch normalised := strings.ToLower(strings.TrimSpace(*conn.String())); normalised {
		case "", "default":
		case "on", "true":
			think = true
		case "off", "false":
			think = false
		case "low", "medium", "high":
			think = normalised
		default:
			log.WithFields(log.Fields{
				"value": *conn.String(),
			}).Warn("[ollama] invalid think value; using model default")
		}
	}

	keepAlive := ""
	if conn := core.FindConnection("keep_alive", inputs); conn != nil && conn.String() != nil {
		keepAlive = strings.TrimSpace(*conn.String())
	}

	systemPromptStr := ""
	if conn := core.FindConnection("system_prompt", inputs); conn != nil && conn.String() != nil && *conn.String() != "" {
		systemPromptStr = *conn.String()
	}
	// Teach the model to pass flo:blob:<handle> tokens verbatim to
	// downstream tools rather than inventing placeholder strings for
	// large outputs it can't see. See ai_common for the rationale.
	systemPromptStr = ai_common.AppendBlobTokenInstructions(systemPromptStr)

	tools := parseToolDefinitions(inputs)

	// Check if we're in a tool loop (re-invocation with tool results)
	var messages []interface{}
	toolCallsCount := 0

	if convState, ok := flow.GetVariable(core.ToolConversationStateKey); ok && convState != nil {
		if ms, ok := convState.([]interface{}); ok {
			messages = ms
		}

		// Append tool results. Ollama matches results to calls by tool
		// name rather than id, so map each synthesised id back to the
		// name recorded in the assistant message's tool_calls.
		if toolResults, ok := flow.GetVariable(core.ToolResultsKey); ok && toolResults != nil {
			if results, ok := toolResults.([]core.ToolResult); ok {
				idToName := lastToolCallNames(messages)
				for _, r := range results {
					msg := map[string]interface{}{
						"role":    "tool",
						"content": r.Content,
					}
					if name, ok := idToName[r.ToolUseID]; ok && name != "" {
						msg["tool_name"] = name
					}
					messages = append(messages, msg)
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

		if historyConn := core.FindConnection("conversation_history", inputs); historyConn != nil {
			history := ai_common.ParseConversationHistory(historyConn.Value)
			if len(history) > 0 {
				budgetMaxTokens := int64(defaultMaxTokens)
				if maxTokens != nil && *maxTokens > 0 {
					budgetMaxTokens = *maxTokens
				}
				// The real context window is whatever num_ctx the server
				// loads the model with; an explicit num_ctx input is a
				// better truncation budget than the model-name heuristic.
				contextWindow := ai_common.ModelContextWindow(model)
				if numCtx != nil && *numCtx > 0 {
					contextWindow = int(*numCtx)
				}
				history = ai_common.TruncateHistoryForBudget(
					history, systemPromptStr, prompt,
					int(budgetMaxTokens), contextWindow,
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

		// Vision: resolve [attached: ...] image markers to blob bytes and
		// attach them as base64 strings on the user message — Ollama's
		// native image shape (not OpenAI image_url blocks).
		messages = append(messages, buildUserMessage(prompt, flow.Blobs()))
	}

	payload := map[string]interface{}{
		"model":    model,
		"messages": messages,
		// /api/chat streams by default; this action consumes a single
		// JSON response.
		"stream": false,
	}
	if len(options) > 0 {
		payload["options"] = options
	}
	if format != nil {
		payload["format"] = format
	}
	if keepAlive != "" {
		// The server parses string values with Go's ParseDuration, so the
		// bare-number spellings ("300" = seconds, "-1" = forever) must go
		// as JSON numbers — as strings they fail the whole request bind
		// with HTTP 400 ("missing unit in duration").
		if n, err := strconv.ParseFloat(keepAlive, 64); err == nil {
			payload["keep_alive"] = n
		} else {
			payload["keep_alive"] = keepAlive
		}
	}
	if think != nil {
		payload["think"] = think
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
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))

	if resp.StatusCode != http.StatusOK {
		// Native error shape is {"error": "..."}; proxies may wrap it
		// OpenAI-style as {"error": {"message": "..."}}.
		var apiErr struct {
			Error json.RawMessage `json:"error"`
		}
		errMsg := ""
		if json.Unmarshal(respBody, &apiErr) == nil && len(apiErr.Error) > 0 {
			var s string
			var o struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(apiErr.Error, &s) == nil {
				errMsg = s
			} else if json.Unmarshal(apiErr.Error, &o) == nil {
				errMsg = o.Message
			}
		}
		if errMsg == "" {
			errMsg = string(respBody)
		}
		return nil, fmt.Errorf("Ollama API error (%d): %s", resp.StatusCode, errMsg)
	}

	var result struct {
		Model   string `json:"model"`
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			Thinking  string `json:"thinking"`
			ToolCalls []struct {
				Function struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		DoneReason      string `json:"done_reason"`
		PromptEvalCount int64  `json:"prompt_eval_count"`
		EvalCount       int64  `json:"eval_count"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}

	modelUsed := result.Model
	if modelUsed == "" {
		modelUsed = model
	}
	totalTokens := result.PromptEvalCount + result.EvalCount

	if result.DoneReason == "length" {
		log.WithFields(log.Fields{
			"model":      modelUsed,
			"max_tokens": maxTokens,
		}).Warn("[ollama] response truncated by the token limit (done_reason=length)")
	}

	// Tool calls. Unlike the OpenAI family there is no finish_reason
	// signal to key on — the presence of tool_calls is the signal.
	if len(result.Message.ToolCalls) > 0 {
		var toolRequests []core.ToolRequest
		var stateCalls []interface{}

		for i, tc := range result.Message.ToolCalls {
			input := tc.Function.Arguments
			if input == nil {
				input = map[string]interface{}{}
			}
			// Ollama tool calls carry no id; synthesise one, unique
			// across rounds, for the engine's result round-trip. The id
			// is also kept in the conversation state (the server ignores
			// unknown fields) so the re-entry leg can map results back
			// to tool names.
			id := fmt.Sprintf("call_%d_%d", len(messages), i)
			toolRequests = append(toolRequests, core.ToolRequest{
				ID:    id,
				Name:  tc.Function.Name,
				Input: input,
			})
			stateCalls = append(stateCalls, map[string]interface{}{
				"id":   id,
				"type": "function",
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": input,
				},
			})
		}

		assistantMsg := map[string]interface{}{
			"role":       "assistant",
			"content":    result.Message.Content,
			"tool_calls": stateCalls,
		}
		// Thinking-model chat templates re-render prior assistant thinking
		// for turns after the last user message — exactly where this
		// message sits on the re-entry leg — so dropping it degrades
		// multi-round tool use.
		if result.Message.Thinking != "" {
			assistantMsg["thinking"] = result.Message.Thinking
		}
		messages = append(messages, assistantMsg)

		out := map[string]interface{}{
			core.ToolRequestsKey:          toolRequests,
			core.ToolConversationStateKey: messages,
			"stop_reason":                 "tool_calls",
			"model":                       modelUsed,
			"thinking":                    result.Message.Thinking,
			"prompt_tokens":               result.PromptEvalCount,
			"completion_tokens":           result.EvalCount,
			"total_tokens":                totalTokens,
			"tool_calls_count":            len(toolRequests),
			"success":                     true,
			"error":                       "",
		}
		// Capture any text the model emitted alongside tool calls so the
		// engine can send it to the user via the Response handle mid-loop.
		if result.Message.Content != "" {
			out[core.IntermediateTextKey] = result.Message.Content
		}
		return out, nil
	}

	// Final text response
	content := result.Message.Content

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
		"thinking":          result.Message.Thinking,
		"should_respond":    shouldRespond,
		"model":             modelUsed,
		"prompt_tokens":     result.PromptEvalCount,
		"completion_tokens": result.EvalCount,
		"total_tokens":      totalTokens,
		"tool_calls_count":  toolCallsCount,
		"success":           true,
		"error":             "",
	}, nil
}

// buildUserMessage assembles the user message, promoting any [attached: ...]
// image markers in the prompt into Ollama's native images field: an array of
// bare base64 strings alongside the text content.
func buildUserMessage(prompt string, fetcher ai_common.BlobFetcher) map[string]interface{} {
	stripped, blobs := ai_common.ExtractVisionBlobs(prompt, fetcher)
	msg := map[string]interface{}{
		"role":    "user",
		"content": stripped,
	}
	if len(blobs) > 0 {
		images := make([]string, 0, len(blobs))
		for _, b := range blobs {
			images = append(images, base64.StdEncoding.EncodeToString(b.Bytes))
		}
		msg["images"] = images
	}
	return msg
}

// parseToolDefinitions reads the tool_definitions input and normalises each
// entry to Ollama's tool schema. Two shapes are accepted: OpenAI-style
// {type: "function", function: {name, description, parameters}} (manual
// input, matching the other AI actions) and Anthropic-style
// {name, description, input_schema} (what the engine's Tools-handle
// auto-discovery injects).
func parseToolDefinitions(inputs []*core.Connection) []interface{} {
	conn := core.FindConnection("tool_definitions", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return nil
	}
	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(*conn.String()), &raw); err != nil {
		return nil
	}
	tools := make([]interface{}, 0, len(raw))
	for _, entry := range raw {
		if _, ok := entry["function"]; ok {
			tools = append(tools, entry)
			continue
		}
		name, _ := entry["name"].(string)
		if name == "" {
			continue
		}
		fn := map[string]interface{}{"name": name}
		if desc, ok := entry["description"].(string); ok && desc != "" {
			fn["description"] = desc
		}
		if schema, ok := entry["input_schema"]; ok {
			fn["parameters"] = schema
		}
		tools = append(tools, map[string]interface{}{
			"type":     "function",
			"function": fn,
		})
	}
	if len(tools) == 0 {
		return nil
	}
	return tools
}

// lastToolCallNames maps synthesised tool-call ids to tool names from the
// most recent assistant message carrying tool_calls, so tool results can be
// echoed back with the tool_name Ollama matches on.
func lastToolCallNames(messages []interface{}) map[string]string {
	idToName := map[string]string{}
	for i := len(messages) - 1; i >= 0; i-- {
		m, ok := messages[i].(map[string]interface{})
		if !ok || m["role"] != "assistant" {
			continue
		}
		calls, ok := m["tool_calls"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range calls {
			call, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := call["id"].(string)
			fn, _ := call["function"].(map[string]interface{})
			name, _ := fn["name"].(string)
			if id != "" && name != "" {
				idToName[id] = name
			}
		}
		break
	}
	return idToName
}

// optionalFloat parses an optional string-typed numeric input. Returns nil
// when the input is absent or blank; malformed values warn and are omitted
// from the request so model defaults apply.
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
		}).Warn("[ollama] invalid number; omitting from request")
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
