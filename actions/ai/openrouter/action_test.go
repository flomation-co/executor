package openrouter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// chatResponse is a minimal OpenAI-compatible Chat Completions response body
// (OpenRouter mirrors the OpenAI shape and adds provider + usage.cost).
func chatResponse(model, content, finishReason string) map[string]interface{} {
	return map[string]interface{}{
		"id":       "gen-test",
		"model":    model,
		"provider": "TestProvider",
		"usage": map[string]interface{}{
			"prompt_tokens":     10,
			"completion_tokens": 20,
			"total_tokens":      30,
			"cost":              0.000123,
		},
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": finishReason,
			},
		},
	}
}

// withServer points the package apiURL at a test server for the duration of
// the test, restoring the real OpenRouter endpoint afterwards.
func withServer(url string) func() {
	prev := apiURL
	apiURL = url
	return func() { apiURL = prev }
}

func TestExecuteMissingAPIKey(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "model", Type: core.ConnectionTypeString, Value: "openai/gpt-5.4-mini"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("api_key is required"))
	Expect(result).To(BeNil())
}

func TestExecuteMissingPrompt(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "openai/gpt-5.4-mini"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("prompt is required"))
	Expect(result).To(BeNil())
}

func TestExecuteChatCompletion(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bearer auth, POST, JSON content type, attribution headers.
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer sk-or-test-key"))
		Expect(r.Header.Get("HTTP-Referer")).To(Equal("https://www.flomation.co"))
		Expect(r.Header.Get("X-Title")).To(Equal("Flomation"))
		Expect(r.Method).To(Equal(http.MethodPost))

		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		Expect(reqBody["model"]).To(Equal("anthropic/claude-haiku-4.5"))

		messages := reqBody["messages"].([]interface{})
		Expect(len(messages)).To(Equal(2)) // system + user

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("anthropic/claude-haiku-4.5", "Hello from OpenRouter!", "stop"))
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "anthropic/claude-haiku-4.5"},
		{Name: "system_prompt", Type: core.ConnectionTypeText, Value: "You are helpful."},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hi there"},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result).To(Not(BeNil()))
	Expect(result["response"]).To(Equal("Hello from OpenRouter!"))
	Expect(result["model"]).To(Equal("anthropic/claude-haiku-4.5"))
	Expect(result["provider"]).To(Equal("TestProvider"))
	Expect(result["cost"]).To(Equal("0.000123"))
	Expect(result["total_tokens"]).To(Equal(int64(30)))
	Expect(result["should_respond"]).To(Equal(true))
	Expect(result["success"]).To(Equal(true))
}

func TestExecuteDefaultModel(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("openai/gpt-5.4-mini", "ok", "stop"))
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hello"},
	}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	// No model supplied → falls back to the flagship default.
	Expect(reqBody["model"]).To(Equal("openai/gpt-5.4-mini"))
}

// TestExecuteSamplingParamsOmittedWhenUnset pins the contract that optional
// sampling controls are left out of the request entirely unless the user set
// them — some models behind OpenRouter reject sampling params they don't
// support, so sending defaults would break those models.
func TestExecuteSamplingParamsOmittedWhenUnset(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("openai/gpt-5.4-mini", "ok", "stop"))
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hello"},
	}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(reqBody).To(Not(HaveKey("top_p")))
	Expect(reqBody).To(Not(HaveKey("frequency_penalty")))
	Expect(reqBody).To(Not(HaveKey("presence_penalty")))
	Expect(reqBody).To(Not(HaveKey("response_format")))
	// Temperature is opt-in too — omitted unless the author set one, for the
	// exact reason above (models that reject unsupported sampling params).
	Expect(reqBody).To(Not(HaveKey("temperature")))
	Expect(reqBody["max_tokens"]).To(Equal(float64(2048)))
}

func TestExecuteSamplingParamsSentWhenSet(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("openai/gpt-5.4-mini", "ok", "stop"))
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Return json please"},
		{Name: "top_p", Type: core.ConnectionTypeString, Value: "0.9"},
		{Name: "frequency_penalty", Type: core.ConnectionTypeString, Value: "0.5"},
		{Name: "presence_penalty", Type: core.ConnectionTypeString, Value: "-0.5"},
		{Name: "response_format", Type: core.ConnectionTypeString, Value: "json_object"},
	}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(reqBody["top_p"]).To(Equal(0.9))
	Expect(reqBody["frequency_penalty"]).To(Equal(0.5))
	Expect(reqBody["presence_penalty"]).To(Equal(-0.5))
	Expect(reqBody["response_format"]).To(Equal(map[string]interface{}{"type": "json_object"}))
}

// TestExecuteResponseFormatNormalised pins the response_format contract for
// values arriving via ${...} substitution rather than the editor dropdown:
// "JSON"/"json" normalise to json_object, unrecognised values fall back to
// text mode (omitted) with a warning rather than silently changing behavior.
func TestExecuteResponseFormatNormalised(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("openai/gpt-5.4-mini", "{}", "stop"))
	}))
	defer server.Close()
	defer withServer(server.URL)()

	base := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Return json please"},
	}

	// Case-insensitive alias: "JSON" → json_object.
	_, err := Execute(&core.Flow{}, nil, append(base,
		&core.Connection{Name: "response_format", Type: core.ConnectionTypeString, Value: "JSON"}))
	Expect(err).To(BeNil())
	Expect(reqBody["response_format"]).To(Equal(map[string]interface{}{"type": "json_object"}))

	// Unrecognised value → omitted (text mode), not sent verbatim.
	reqBody = nil // Decode merges into an existing map; reset between requests.
	_, err = Execute(&core.Flow{}, nil, append(base,
		&core.Connection{Name: "response_format", Type: core.ConnectionTypeString, Value: "yaml"}))
	Expect(err).To(BeNil())
	Expect(reqBody).To(Not(HaveKey("response_format")))
}

func TestExecuteMalformedSamplingParamOmitted(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("openai/gpt-5.4-mini", "ok", "stop"))
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hello"},
		{Name: "top_p", Type: core.ConnectionTypeString, Value: "not-a-number"},
	}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	// Malformed optional param must be omitted, not sent as 0.
	Expect(reqBody).To(Not(HaveKey("top_p")))
}

func TestExecuteConversationHistory(t *testing.T) {
	RegisterTestingT(t)

	var capturedMessages []interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		capturedMessages = reqBody["messages"].([]interface{})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("openai/gpt-5.4-mini", "answer", "stop"))
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "openai/gpt-5.4-mini"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "And after that?"},
		{Name: "conversation_history", Type: core.ConnectionTypeObject, Value: []map[string]interface{}{
			{"role": "user", "content": "First question"},
			{"role": "assistant", "content": "First answer"},
		}},
	}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	// blob-instruction system message (1) + history (2) + new user turn (1).
	// Even with no user-supplied system prompt, the action injects the
	// blob-token guidance as a system message, matching ai/openai.
	Expect(len(capturedMessages)).To(Equal(4))
	first := capturedMessages[0].(map[string]interface{})
	Expect(first["role"]).To(Equal("system"))
	last := capturedMessages[3].(map[string]interface{})
	Expect(last["role"]).To(Equal("user"))
	Expect(last["content"]).To(Equal("And after that?"))
}

func TestExecuteToolCalls(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		Expect(reqBody["tools"]).To(Not(BeNil()))

		resp := map[string]interface{}{
			"id":       "gen-tool",
			"model":    "openai/gpt-5.4-mini",
			"provider": "TestProvider",
			"usage":    map[string]interface{}{"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10, "cost": 0.00001},
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role": "assistant",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "web_search",
									"arguments": `{"query":"weather"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "openai/gpt-5.4-mini"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "What's the weather?"},
		{Name: "tool_definitions", Type: core.ConnectionTypeText, Value: `[{"type":"function","function":{"name":"web_search","description":"Search","parameters":{"type":"object","properties":{"query":{"type":"string"}}}}}]`},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["stop_reason"]).To(Equal("tool_calls"))
	Expect(result["tool_calls_count"]).To(Equal(1))

	requests, ok := result[core.ToolRequestsKey].([]core.ToolRequest)
	Expect(ok).To(BeTrue())
	Expect(len(requests)).To(Equal(1))
	Expect(requests[0].ID).To(Equal("call_1"))
	Expect(requests[0].Name).To(Equal("web_search"))
	Expect(requests[0].Input).To(HaveKeyWithValue("query", "weather"))
	Expect(result[core.ToolConversationStateKey]).To(Not(BeNil()))
}

// TestExecuteEmptyToolArgumentsNormalised covers a known quirk of models
// reached through OpenRouter (historically Anthropic): tools that take no
// parameters can come back with `"arguments": ""` instead of `"{}"`. The
// action must normalise so the parsed input is an empty map and the
// conversation state echoes "{}" back to the provider on the next round.
func TestExecuteEmptyToolArgumentsNormalised(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":       "gen-tool",
			"model":    "anthropic/claude-haiku-4.5",
			"provider": "TestProvider",
			"usage":    map[string]interface{}{"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10, "cost": 0.00001},
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role": "assistant",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_time",
									"arguments": "",
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "anthropic/claude-haiku-4.5"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "What time is it?"},
		{Name: "tool_definitions", Type: core.ConnectionTypeText, Value: `[{"type":"function","function":{"name":"get_time","description":"Get the time","parameters":{"type":"object","properties":{}}}}]`},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())

	requests, ok := result[core.ToolRequestsKey].([]core.ToolRequest)
	Expect(ok).To(BeTrue())
	Expect(len(requests)).To(Equal(1))
	// Empty arguments must parse to an empty (non-nil) input map.
	Expect(requests[0].Input).To(Not(BeNil()))
	Expect(requests[0].Input).To(BeEmpty())

	// The conversation state must carry "{}" — providers reject "" when
	// the assistant message is echoed back on the next round.
	stateJSON, marshalErr := json.Marshal(result[core.ToolConversationStateKey])
	Expect(marshalErr).To(BeNil())
	Expect(string(stateJSON)).To(ContainSubstring(`"arguments":"{}"`))
}

// TestExecuteToolResultReinvocation covers the second leg of the tool
// loop: the engine re-invokes Execute with the prior conversation state
// and the executed tool results, and the action must append those results
// as `tool` messages and return the model's final text answer.
func TestExecuteToolResultReinvocation(t *testing.T) {
	RegisterTestingT(t)

	var capturedMessages []interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		capturedMessages = reqBody["messages"].([]interface{})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("openai/gpt-5.4-mini", "It is sunny.", "stop"))
	}))
	defer server.Close()
	defer withServer(server.URL)()

	flow := &core.Flow{}
	// Prior turn's assistant message that requested the tool call. The
	// action stores conversation state as []interface{}.
	flow.SetVariable(core.ToolConversationStateKey, []interface{}{
		map[string]interface{}{"role": "user", "content": "What's the weather?"},
		map[string]interface{}{
			"role": "assistant",
			"tool_calls": []map[string]interface{}{
				{"id": "call_1", "type": "function", "function": map[string]interface{}{"name": "web_search", "arguments": `{"query":"weather"}`}},
			},
		},
	})
	// Engine-supplied results of executing those tool calls.
	flow.SetVariable(core.ToolResultsKey, []core.ToolResult{
		{ToolUseID: "call_1", Content: "sunny, 25C"},
	})

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "openai/gpt-5.4-mini"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "What's the weather?"},
	}

	result, err := Execute(flow, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["response"]).To(Equal("It is sunny."))
	Expect(result["should_respond"]).To(Equal(true))

	// The tool result must be appended as a `tool` message carrying the
	// original tool_call_id, after the restored conversation state.
	Expect(len(capturedMessages)).To(Equal(3)) // user + assistant + tool
	toolMsg := capturedMessages[2].(map[string]interface{})
	Expect(toolMsg["role"]).To(Equal("tool"))
	Expect(toolMsg["tool_call_id"]).To(Equal("call_1"))
	Expect(toolMsg["content"]).To(Equal("sunny, 25C"))
}

func TestExecuteInvalidTemperatureFallsBack(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("openai/gpt-5.4-mini", "ok", "stop"))
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "openai/gpt-5.4-mini"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hello"},
		{Name: "temperature", Type: core.ConnectionTypeString, Value: "not-a-number"},
	}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	// Temperature is opt-in: a malformed value is omitted entirely rather than
	// coerced to a default that a routed model might reject.
	Expect(reqBody).To(Not(HaveKey("temperature")))
}

func TestExecuteNoResponseSentinel(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("openai/gpt-5.4-mini", "[NO_RESPONSE]", "stop"))
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "openai/gpt-5.4-mini"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "stay quiet"},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["should_respond"]).To(Equal(false))
	Expect(result["response"]).To(Equal(""))
}

func TestExecuteAPIError(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid API Key",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()
	defer withServer(server.URL)()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-or-bad-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "openai/gpt-5.4-mini"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hello"},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Invalid API Key"))
	Expect(result).To(BeNil())
}
