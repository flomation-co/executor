// Package gemini implements the Gemini Prompt action — text and
// multi-turn chat against Google's Gemini API (generateContent).
//
// Modelled on the OpenAI and Anthropic prompt actions so the engine's
// tool-loop, conversation-history recording, and vision-block promotion
// all flow through unchanged. The provider differences are isolated to
// the request/response shapes inside Execute:
//
//   - Auth via x-goog-api-key header (cleaner than the alternative
//     ?key= query param, and avoids leaking the key in proxy access
//     logs).
//   - System prompt lives at the top level as systemInstruction,
//     separate from the contents array.
//   - Contents is a list of {role, parts[]} where parts can be text,
//     inline_data (image bytes for vision-in), or function_call /
//     function_response for tool use.
//   - Roles are "user" and "model" (not "assistant"), with no system
//     role inside contents.
//
// Like the other AI actions, this one accepts the existing flow-scoped
// tool loop machinery: when re-invoked with __tool_results, it slots
// the prior function_call exchange + new function_response parts back
// into contents and re-asks the model.
package gemini

import (
	"bytes"
	"encoding/base64"
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
	Name         = "Gemini Prompt"
	Description  = "Send a prompt to Google's Gemini API and return the response"
	Website      = "https://www.flomation.co"
	Icon         = "brain+play"
	Date         = "25/06/2026"
	Type         = core.ActionTypeAction

	defaultModel     = "gemini-2.5-flash"
	defaultMaxTokens = 8192
	maxResponseBody  = 1 << 20 // 1 MB
)

// apiBase is a var (not a const) so tests can point it at a
// httptest.Server URL. Production code never mutates this.
var apiBase = "https://generativelanguage.googleapis.com/v1beta/models/"

var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Key",
		Placeholder: "AIza...",
		Required:    true,
	},
	{
		Name:  "model",
		Type:  core.ConnectionTypeString,
		Label: "Model",
		// Static fallback list — the editor replaces this with a live
		// dropdown fetched from the Gemini models endpoint (see the api's
		// getGeminiModels proxy) whenever an API key is set.
		Options: []core.ConnectionOption{
			{Name: "Gemini 2.5 Pro", Value: "gemini-2.5-pro"},
			{Name: "Gemini 2.5 Flash", Value: "gemini-2.5-flash"},
			{Name: "Gemini 2.5 Flash Lite", Value: "gemini-2.5-flash-lite"},
			{Name: "Gemini 2.0 Flash", Value: "gemini-2.0-flash"},
			{Name: "Gemini 2.0 Flash Lite", Value: "gemini-2.0-flash-lite"},
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
		Placeholder: "8192",
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
		// Tool definitions passed as JSON. Accepts both Gemini's native
		// shape ({"functionDeclarations":[{...}]}) and OpenAI's shape
		// ([{"type":"function","function":{...}}]) — the latter is
		// transparently converted so flows can share tool defs across
		// providers without rewriting them.
		Name:        "tool_definitions",
		Type:        core.ConnectionTypeText,
		Label:       "Tool Definitions (JSON)",
		Placeholder: `[{"name":"web_search","description":"Search the web","parameters":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}]`,
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

// geminiPart is the union shape Gemini's API uses for content. Only one
// of Text / InlineData / FunctionCall / FunctionResponse is populated
// per part. We keep it loose (interface{}) for inline_data because we
// build it as a map literal at marshal time.
type geminiPart struct {
	Text             string                 `json:"text,omitempty"`
	InlineData       *geminiInlineData      `json:"inline_data,omitempty"`
	FunctionCall     *geminiFunctionCall    `json:"functionCall,omitempty"`
	FunctionResponse *geminiFnResponseShape `json:"functionResponse,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // base64
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args,omitempty"`
}

type geminiFnResponseShape struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
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
	if c := core.FindConnection("model", inputs); c != nil && c.String() != nil && *c.String() != "" {
		model = *c.String()
	}

	maxTokens := int64(defaultMaxTokens)
	if c := core.FindConnection("max_tokens", inputs); c != nil && c.Number() != nil && *c.Number() > 0 {
		maxTokens = *c.Number()
	}

	temperature := 0.7
	if c := core.FindConnection("temperature", inputs); c != nil && c.String() != nil && *c.String() != "" {
		// Parse best-effort; if the user's value is garbage we keep
		// the default 0.7 rather than failing the whole node.
		_, _ = fmt.Sscanf(*c.String(), "%f", &temperature)
	}

	systemPromptStr := ""
	if c := core.FindConnection("system_prompt", inputs); c != nil && c.String() != nil && *c.String() != "" {
		systemPromptStr = *c.String()
	}
	systemPromptStr = ai_common.AppendBlobTokenInstructions(systemPromptStr)

	// Parse tool definitions if provided. Accept both Gemini's native
	// shape and OpenAI's shape so the same flow can swap providers
	// without rewriting the tool defs.
	var functionDeclarations []interface{}
	if c := core.FindConnection("tool_definitions", inputs); c != nil {
		var raw string
		if s := c.String(); s != nil && *s != "" {
			raw = strings.TrimSpace(*s)
		}
		if raw != "" {
			functionDeclarations = parseToolDefinitions(raw)
		}
	}

	// Resume tool loop or build a fresh contents array.
	var contents []geminiContent
	toolCallsCount := 0

	if convState, ok := flow.GetVariable(core.ToolConversationStateKey); ok && convState != nil {
		if cs, ok := convState.([]geminiContent); ok {
			contents = cs
		} else if csi, ok := convState.([]interface{}); ok {
			// JSON-roundtripped from an earlier turn — rebuild typed.
			contents = reviveContents(csi)
		}

		// Append tool results as a user message with function_response
		// parts — Gemini's spec requires them under the user role. The
		// `name` field on each response must match the function_call's
		// name; ToolResult only carries ToolUseID, so we recover the
		// name from the ID we constructed in the previous turn
		// ("%s-%d" — see the function_call response handling further
		// down).
		if toolResults, ok := flow.GetVariable(core.ToolResultsKey); ok && toolResults != nil {
			if results, ok := toolResults.([]core.ToolResult); ok && len(results) > 0 {
				parts := make([]geminiPart, 0, len(results))
				for _, r := range results {
					name := r.ToolUseID
					if i := strings.LastIndex(name, "-"); i > 0 {
						name = name[:i]
					}
					content := map[string]interface{}{"content": r.Content}
					if r.IsError {
						content["error"] = true
					}
					parts = append(parts, geminiPart{
						FunctionResponse: &geminiFnResponseShape{
							Name:     name,
							Response: content,
						},
					})
				}
				contents = append(contents, geminiContent{Role: "user", Parts: parts})
			}
		}
	} else {
		// First invocation — system prompt is carried separately
		// (systemInstruction at the top level), so contents starts with
		// the prior conversation history followed by this turn's user
		// message.
		if c := core.FindConnection("conversation_history", inputs); c != nil {
			history := ai_common.ParseConversationHistory(c.Value)
			if len(history) > 0 {
				history = ai_common.TruncateHistoryForBudget(
					history, systemPromptStr, prompt,
					int(maxTokens), ai_common.ModelContextWindow(model),
				)
				for _, m := range history {
					if m.Role == "" || m.Content == "" {
						continue
					}
					role := m.Role
					if role == "assistant" {
						role = "model"
					}
					contents = append(contents, geminiContent{
						Role:  role,
						Parts: []geminiPart{{Text: m.Content}},
					})
				}
			}
		}

		// Vision-block promotion: convert [attached:] markers in the
		// prompt text into inline_data parts using the blob bytes.
		userParts := buildGeminiUserParts(prompt, flow.Blobs())
		contents = append(contents, geminiContent{Role: "user", Parts: userParts})
	}

	// Streaming: fire the Response handle per sentence for low-latency
	// voice. Skipped on tool re-invocations — the tool loop needs the full
	// response to detect function calls before deciding whether to stream.
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
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     temperature,
			"maxOutputTokens": maxTokens,
		},
	}
	if systemPromptStr != "" {
		payload["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{{"text": systemPromptStr}},
		}
	}
	if len(functionDeclarations) > 0 {
		payload["tools"] = []map[string]interface{}{
			{"functionDeclarations": functionDeclarations},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Streaming uses the SSE variant of the endpoint; the shared Gemini
	// stream parser reads it. Non-streaming hits :generateContent.
	endpoint := ":generateContent"
	if useStreaming {
		endpoint = ":streamGenerateContent?alt=sse"
	}
	url := apiBase + model + endpoint
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respBody, &apiErr)
		errMsg := apiErr.Error.Message
		if errMsg == "" {
			errMsg = string(respBody)
		}
		return nil, fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, errMsg)
	}

	// Streaming response: the shared Gemini SSE parser owns resp.Body and
	// emits sentences via the engine's streaming channel contract.
	if useStreaming {
		return ai_common.HandleGeminiStream(flow, resp, model, map[string]interface{}{
			"prompt_tokens":     int64(0),
			"completion_tokens": int64(0),
			"total_tokens":      int64(0),
		})
	}

	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         *string             `json:"text,omitempty"`
					FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
				} `json:"parts"`
				Role string `json:"role"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		ModelVersion  string `json:"modelVersion"`
		UsageMetadata struct {
			PromptTokenCount     int64 `json:"promptTokenCount"`
			CandidatesTokenCount int64 `json:"candidatesTokenCount"`
			TotalTokenCount      int64 `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}
	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("Gemini returned no candidates")
	}

	cand := result.Candidates[0]
	resolvedModel := result.ModelVersion
	if resolvedModel == "" {
		resolvedModel = model
	}

	// Collect tool calls and any inline text the model emitted alongside.
	var toolRequests []core.ToolRequest
	var inlineText strings.Builder
	for _, p := range cand.Content.Parts {
		if p.Text != nil && *p.Text != "" {
			inlineText.WriteString(*p.Text)
		}
		if p.FunctionCall != nil && p.FunctionCall.Name != "" {
			// Gemini doesn't give function calls a stable ID. Use the
			// name + index so the engine can correlate request to
			// response within this turn.
			id := fmt.Sprintf("%s-%d", p.FunctionCall.Name, len(toolRequests))
			toolRequests = append(toolRequests, core.ToolRequest{
				ID:    id,
				Name:  p.FunctionCall.Name,
				Input: p.FunctionCall.Args,
			})
		}
	}

	if len(toolRequests) > 0 {
		// Persist the model's message (function_call parts) so the next
		// turn can append function_response parts in the right place.
		modelParts := make([]geminiPart, 0, len(cand.Content.Parts))
		for _, p := range cand.Content.Parts {
			gp := geminiPart{}
			if p.Text != nil {
				gp.Text = *p.Text
			}
			if p.FunctionCall != nil {
				gp.FunctionCall = p.FunctionCall
			}
			modelParts = append(modelParts, gp)
		}
		contents = append(contents, geminiContent{Role: "model", Parts: modelParts})

		out := map[string]interface{}{
			core.ToolRequestsKey:          toolRequests,
			core.ToolConversationStateKey: contents,
			"stop_reason":                 "function_call",
			"model":                       resolvedModel,
			"prompt_tokens":               result.UsageMetadata.PromptTokenCount,
			"completion_tokens":           result.UsageMetadata.CandidatesTokenCount,
			"total_tokens":                result.UsageMetadata.TotalTokenCount,
			"tool_calls_count":            len(toolRequests),
			"success":                     true,
			"error":                       "",
		}
		if text := strings.TrimSpace(inlineText.String()); text != "" {
			out[core.IntermediateTextKey] = text
		}
		return out, nil
	}

	// Final text response.
	content := inlineText.String()
	shouldRespond := true
	trimmed := strings.TrimSpace(content)
	if trimmed == "[NO_RESPONSE]" || strings.Contains(trimmed, "[NO_RESPONSE]") {
		shouldRespond = false
		content = ""
	}

	if shouldRespond && content != "" {
		if exchanges := extractToolExchanges(flow); len(exchanges) > 0 {
			ai_common.RecordToolExchange(flow.GoContext(), flow.GetContext(), exchanges)
			toolCallsCount = len(exchanges)
		}
		ai_common.RecordAssistantReply(flow.GoContext(), flow.GetContext(), content)
	}

	return map[string]interface{}{
		"response":          content,
		"should_respond":    shouldRespond,
		"model":             resolvedModel,
		"prompt_tokens":     result.UsageMetadata.PromptTokenCount,
		"completion_tokens": result.UsageMetadata.CandidatesTokenCount,
		"total_tokens":      result.UsageMetadata.TotalTokenCount,
		"tool_calls_count":  toolCallsCount,
		"success":           true,
		"error":             "",
	}, nil
}

// buildGeminiUserParts converts a prompt string into Gemini parts,
// promoting any [attached: name (mime, size) → flo:blob:HANDLE]
// image markers into inline_data parts. Non-image attachments keep
// their marker in the text — same scope as the OpenAI vision handler.
func buildGeminiUserParts(prompt string, fetcher ai_common.BlobFetcher) []geminiPart {
	stripped, images := ai_common.ExtractVisionBlobs(prompt, fetcher)
	if len(images) == 0 {
		return []geminiPart{{Text: prompt}}
	}
	parts := make([]geminiPart, 0, len(images)+1)
	if stripped != "" {
		parts = append(parts, geminiPart{Text: stripped})
	}
	for _, img := range images {
		parts = append(parts, geminiPart{
			InlineData: &geminiInlineData{
				MimeType: img.Mime,
				Data:     base64.StdEncoding.EncodeToString(img.Bytes),
			},
		})
	}
	return parts
}

// parseToolDefinitions accepts both Gemini's native shape and OpenAI's
// shape. The native shape is a list of {name, description, parameters}
// objects; OpenAI wraps each as {"type":"function","function":{...}}.
// Returns the unwrapped flat list of functionDeclarations regardless of
// input shape, or nil on parse failure (the caller treats nil as
// "no tools" — better than failing the action for a malformed JSON
// string the user can fix without restarting).
func parseToolDefinitions(raw string) []interface{} {
	var arr []interface{}
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		// Allow Gemini's wrapped object form too:
		// {"functionDeclarations":[...]}.
		var wrapped struct {
			FunctionDeclarations []interface{} `json:"functionDeclarations"`
		}
		if err := json.Unmarshal([]byte(raw), &wrapped); err != nil {
			return nil
		}
		return wrapped.FunctionDeclarations
	}
	out := make([]interface{}, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		// OpenAI shape — unwrap.
		if t, _ := m["type"].(string); t == "function" {
			if fn, ok := m["function"].(map[string]interface{}); ok {
				out = append(out, fn)
				continue
			}
		}
		// Already in Gemini shape.
		out = append(out, m)
	}
	return out
}

// reviveContents rebuilds a typed []geminiContent from a JSON-decoded
// []interface{} (the shape that comes back after a flow variable
// round-trips through the event bus). Lenient — silently skips
// malformed entries rather than blowing up the tool loop.
func reviveContents(raw []interface{}) []geminiContent {
	out := make([]geminiContent, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		var parts []geminiPart
		if ps, ok := m["parts"].([]interface{}); ok {
			for _, pi := range ps {
				pm, ok := pi.(map[string]interface{})
				if !ok {
					continue
				}
				gp := geminiPart{}
				if t, ok := pm["text"].(string); ok {
					gp.Text = t
				}
				if fc, ok := pm["functionCall"].(map[string]interface{}); ok {
					name, _ := fc["name"].(string)
					args, _ := fc["args"].(map[string]interface{})
					gp.FunctionCall = &geminiFunctionCall{Name: name, Args: args}
				}
				if fr, ok := pm["functionResponse"].(map[string]interface{}); ok {
					name, _ := fr["name"].(string)
					rsp, _ := fr["response"].(map[string]interface{})
					gp.FunctionResponse = &geminiFnResponseShape{Name: name, Response: rsp}
				}
				parts = append(parts, gp)
			}
		}
		out = append(out, geminiContent{Role: role, Parts: parts})
	}
	return out
}

// extractToolExchanges reads the accumulated tool exchanges from the
// flow variable set by the engine's tool loop. Returns nil if no tools
// were called in this turn. Mirrors the helper in the OpenAI and
// Anthropic actions.
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
