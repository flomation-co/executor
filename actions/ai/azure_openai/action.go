// Package azure_openai implements the ai/azure_openai action: a chat-completion
// call against an Azure OpenAI deployment.
//
// The wire format is OpenAI's Chat Completions API, so this package deliberately
// tracks actions/ai/openrouter (which tracks ai/groq → ai/openai) — including the
// tool-calling loop, vision image-block promotion, conversation-history
// truncation, and the [NO_RESPONSE] sentinel. The Azure deltas are all plumbing:
// the key travels in the non-standard "api-key" header (never Authorization:
// Bearer), the model is a customer-named DEPLOYMENT interpolated into the URL
// path rather than a body field, every call needs an api-version query param,
// and the newer structured-output mode (response_format json_schema) is exposed
// because Azure supports it and n8n does not. Azure returns neither a provider
// nor a per-request cost, so those two openrouter outputs are dropped.
package azure_openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	Name         = "Azure OpenAI Prompt"
	Description  = "Send a prompt to an Azure OpenAI deployment and return the response"
	Website      = "https://www.flomation.co"
	Icon         = "brain+cloud"
	Date         = "16/07/2026"
	Type         = core.ActionTypeAction

	defaultMaxTokens = 2048
	maxResponseBody  = 1 << 20 // 1 MB

	// defaultAPIVersion is the current GA data-plane version. Structured
	// outputs (json_schema) shipped in 2024-08-01-preview and are GA here.
	defaultAPIVersion = "2024-10-21"
)

// httpClient is shared across calls so connections are pooled. The timeout
// matches the other AI chat actions' long deadline: reasoning deployments can
// legitimately think for minutes.
var httpClient = &http.Client{Timeout: 360 * time.Second}

// AuthInputs is the canonical auth/target block. The Inputs literal below
// re-declares it verbatim (the manifest generator AST-parses the literal array,
// so it cannot follow this variable); TestAuthBlockDoesNotDrift pins the copy.
var AuthInputs = []core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Key",
		Placeholder: "Key 1 or Key 2 from the Azure OpenAI resource",
		Required:    true,
	},
	{
		Name:        "resource_name",
		Type:        core.ConnectionTypeString,
		Label:       "Resource Name",
		Placeholder: "my-resource — builds https://my-resource.openai.azure.com",
	},
	{
		Name:        "endpoint",
		Type:        core.ConnectionTypeString,
		Label:       "Custom Endpoint",
		Placeholder: "https://westeurope.api.cognitive.microsoft.com — overrides Resource Name",
	},
	{
		Name:        "deployment",
		Type:        core.ConnectionTypeString,
		Label:       "Deployment",
		Placeholder: "The deployment name you chose in Azure, not a model id",
		Required:    true,
	},
	{
		Name:        "api_version",
		Type:        core.ConnectionTypeString,
		Label:       "API Version",
		Placeholder: defaultAPIVersion,
	},
}

var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Key",
		Placeholder: "Key 1 or Key 2 from the Azure OpenAI resource",
		Required:    true,
	},
	{
		Name:        "resource_name",
		Type:        core.ConnectionTypeString,
		Label:       "Resource Name",
		Placeholder: "my-resource — builds https://my-resource.openai.azure.com",
	},
	{
		Name:        "endpoint",
		Type:        core.ConnectionTypeString,
		Label:       "Custom Endpoint",
		Placeholder: "https://westeurope.api.cognitive.microsoft.com — overrides Resource Name",
	},
	{
		// The deployment is a customer-chosen name interpolated into the URL
		// path — NOT a model id. A live dropdown is attached via the api
		// repo's dynamic-options metadata; the input stays free-text so
		// ${...} references and API-created flows keep working.
		Name:        "deployment",
		Type:        core.ConnectionTypeString,
		Label:       "Deployment",
		Placeholder: "The deployment name you chose in Azure, not a model id",
		Required:    true,
	},
	{
		Name:        "api_version",
		Type:        core.ConnectionTypeString,
		Label:       "API Version",
		Placeholder: defaultAPIVersion,
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
		// request entirely when left blank, so deployment defaults apply —
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
		// JSON mode needs the word "json" in the prompt or system prompt for
		// models to reliably comply. JSON Schema is Azure's structured-output
		// mode: the reply is guaranteed to match the schema supplied below.
		Name:  "response_format",
		Type:  core.ConnectionTypeString,
		Label: "Response Format",
		Options: []core.ConnectionOption{
			{Name: "Text", Value: "text"},
			{Name: "JSON", Value: "json_object"},
			{Name: "JSON Schema", Value: "json_schema"},
		},
	},
	{
		Name:        "json_schema",
		Type:        core.ConnectionTypeText,
		Label:       "JSON Schema",
		Placeholder: `{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`,
		Visible:     &core.VisibleWhen{Field: "response_format", Values: []string{"json_schema"}},
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

	deployment := stringInput("deployment", inputs)
	if deployment == "" {
		return nil, fmt.Errorf("deployment is required — the Azure deployment name, not a model id")
	}

	endpoint, err := buildBaseURL(stringInput("endpoint", inputs), stringInput("resource_name", inputs))
	if err != nil {
		return nil, err
	}

	apiVersion := stringInput("api_version", inputs)
	if apiVersion == "" {
		apiVersion = defaultAPIVersion
	}

	requestURL := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		endpoint, url.PathEscape(deployment), url.QueryEscape(apiVersion))

	maxTokens := int64(defaultMaxTokens)
	maxTokensConn := core.FindConnection("max_tokens", inputs)
	if maxTokensConn != nil && maxTokensConn.Number() != nil && *maxTokensConn.Number() > 0 {
		maxTokens = *maxTokensConn.Number()
	}

	// Temperature is a string input because the platform has no float
	// connection type (only Integer); see the action's Inputs. Parse it
	// explicitly so malformed values warn loudly and fall back to the
	// default rather than being silently swallowed.
	temperature := 0.7
	tempConn := core.FindConnection("temperature", inputs)
	if tempConn != nil && tempConn.String() != nil && *tempConn.String() != "" {
		raw := strings.TrimSpace(*tempConn.String())
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			temperature = parsed
		} else {
			log.WithFields(log.Fields{
				"value":   raw,
				"default": temperature,
			}).Warn("[azure_openai] invalid temperature; falling back to default")
		}
	}

	// Optional sampling controls — only sent when the user set a valid
	// value, so deployment defaults apply otherwise.
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
	case "", "text", "json_object", "json_schema":
	case "json":
		responseFormat = "json_object"
	default:
		log.WithFields(log.Fields{
			"value":   responseFormat,
			"default": "text",
		}).Warn("[azure_openai] invalid response_format; falling back to text")
		responseFormat = ""
	}

	// Structured outputs: the schema is parsed here so a typo fails the run
	// with a JSON error naming the input, rather than a 400 from Azure that
	// buries it. strict:true is the whole point of the mode — the reply is
	// guaranteed to match, or the request is rejected.
	var schema interface{}
	if responseFormat == "json_schema" {
		raw := stringInput("json_schema", inputs)
		if raw == "" {
			return nil, fmt.Errorf("json_schema is required when Response Format is JSON Schema")
		}
		if err := json.Unmarshal([]byte(raw), &schema); err != nil {
			return nil, fmt.Errorf("json_schema is not valid JSON: %w", err)
		}
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

	// Parse tool definitions if provided (OpenAI format — Azure OpenAI
	// accepts the same tools/tool_calls schema).
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
					int(maxTokens), ai_common.ModelContextWindow(deployment),
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
		// Azure OpenAI accepts on vision-capable deployments.
		userContent := ai_common.BuildOpenAIUserContent(prompt, flow.Blobs())
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": userContent,
		})
	}

	// No "model" body field: on Azure the deployment in the URL path decides
	// which model runs, and a body model is ignored.
	payload := map[string]interface{}{
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
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
	switch responseFormat {
	case "json_object":
		payload["response_format"] = map[string]interface{}{"type": "json_object"}
	case "json_schema":
		payload["response_format"] = map[string]interface{}{
			"type": "json_schema",
			"json_schema": map[string]interface{}{
				"name":   "response",
				"strict": true,
				"schema": schema,
			},
		}
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Azure's key auth uses the non-standard api-key header; the data plane
	// rejects Authorization: Bearer for keys.
	req.Header.Set("api-key", apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Azure OpenAI request failed: %s", redact(err.Error(), apiKey))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Code    string `json:"code"`
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
		errMsg = redact(errMsg, apiKey)

		// A content-filter 400 is Azure refusing the PROMPT, not a broken
		// call — the flow should be able to branch on it rather than die.
		if resp.StatusCode == http.StatusBadRequest && apiErr.Error.Code == "content_filter" {
			return errorResult(fmt.Sprintf(
				"Azure OpenAI's content filter blocked the prompt: %s", errMsg)), nil
		}

		if apiErr.Error.Code != "" {
			return nil, fmt.Errorf("Azure OpenAI API error (%d): %s: %s", resp.StatusCode, apiErr.Error.Code, errMsg)
		}
		return nil, fmt.Errorf("Azure OpenAI API error (%d): %s", resp.StatusCode, errMsg)
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
		return nil, fmt.Errorf("failed to parse Azure OpenAI response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("Azure OpenAI returned no choices")
	}

	choice := result.Choices[0]

	// The other half of the filter: where a blocked prompt is a 400 (handled
	// above), a blocked COMPLETION is an HTTP 200 carrying finish_reason
	// "content_filter" and content that is null or cut off mid-sentence. The
	// default filter is on for every deployment, so without this branch the
	// action reports success with an empty/truncated response and the flow
	// sends it on with nothing naming the filter. Azure supplies no message on
	// this path, so any partial text is echoed as the only available context.
	if choice.FinishReason == "content_filter" {
		msg := "Azure OpenAI's content filter blocked the response"
		if choice.Message.Content != nil && strings.TrimSpace(*choice.Message.Content) != "" {
			msg += fmt.Sprintf(" — partial content before it was cut off: %s", strings.TrimSpace(*choice.Message.Content))
		}
		return errorResult(msg), nil
	}

	// Check for tool calls
	if choice.FinishReason == "tool_calls" && len(choice.Message.ToolCalls) > 0 {
		var toolRequests []core.ToolRequest

		// Tools that take no parameters can come back with "" arguments
		// instead of the "{}" the OpenAI schema specifies. Normalise before
		// the arguments are parsed or echoed back in the conversation state —
		// the API rejects "" on the follow-up request.
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
		"model":             result.Model,
		"prompt_tokens":     result.Usage.PromptTokens,
		"completion_tokens": result.Usage.CompletionTokens,
		"total_tokens":      result.Usage.TotalTokens,
		"tool_calls_count":  toolCallsCount,
		"success":           true,
		"error":             "",
	}, nil
}

// buildBaseURL resolves the target host. A custom endpoint wins outright
// (regional *.api.cognitive.microsoft.com hosts, sovereign clouds, proxies);
// otherwise the resource name builds the standard host — and because it is
// interpolated into a hostname, its charset is validated first. A resource
// name like "evil.example.com/x?" must never become the host we send the
// operator's key to.
func buildBaseURL(endpoint, resourceName string) (string, error) {
	if endpoint != "" {
		return strings.TrimSuffix(endpoint, "/"), nil
	}
	if resourceName == "" {
		return "", fmt.Errorf("either resource_name or endpoint is required")
	}
	for _, r := range resourceName {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
			return "", fmt.Errorf("resource_name %q contains invalid characters — only letters, digits and hyphens are allowed", resourceName)
		}
	}
	return "https://" + resourceName + ".openai.azure.com", nil
}

// errorResult is the soft-failure shape: returned with a nil error so the
// engine routes it to the error port as data instead of killing the run. It
// carries the full output shape so downstream references stay resolvable.
func errorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"response":          "",
		"should_respond":    false,
		"model":             "",
		"prompt_tokens":     int64(0),
		"completion_tokens": int64(0),
		"total_tokens":      int64(0),
		"tool_calls_count":  0,
		"success":           false,
		"error":             msg,
	}
}

// redact masks the API key wherever a provider or transport error echoes it.
func redact(msg, apiKey string) string {
	if apiKey == "" {
		return msg
	}
	return strings.ReplaceAll(msg, apiKey, "********")
}

// stringInput reads an optional string-ish input, returning "" when unset.
func stringInput(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return ""
	}
	s := conn.String()
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

// optionalFloat parses an optional string-typed numeric input. Returns nil
// when the input is absent or blank; malformed values warn and are omitted
// from the request so deployment defaults apply.
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
		}).Warn("[azure_openai] invalid number; omitting from request")
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
