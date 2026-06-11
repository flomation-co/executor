package openwebui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// chatResponse is a minimal OpenAI-compatible Chat Completions response body.
func chatResponse(model, content, finishReason string) map[string]interface{} {
	return map[string]interface{}{
		"id":    "chatcmpl-test",
		"model": model,
		"usage": map[string]interface{}{
			"prompt_tokens":     10,
			"completion_tokens": 20,
			"total_tokens":      30,
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

func TestChatCompletionsURL(t *testing.T) {
	RegisterTestingT(t)

	// Base URL → Open WebUI chat path appended.
	Expect(chatCompletionsURL("https://owui.example.com")).
		To(Equal("https://owui.example.com/api/chat/completions"))
	// Trailing slash trimmed before appending.
	Expect(chatCompletionsURL("https://owui.example.com/")).
		To(Equal("https://owui.example.com/api/chat/completions"))
	// Surrounding whitespace trimmed.
	Expect(chatCompletionsURL("  https://owui.example.com  ")).
		To(Equal("https://owui.example.com/api/chat/completions"))
	// A full endpoint already ending in /chat/completions is used as-is
	// (supports LiteLLM/vLLM/Ollama-shim style /v1/chat/completions).
	Expect(chatCompletionsURL("https://host/v1/chat/completions")).
		To(Equal("https://host/v1/chat/completions"))
}

func TestExecuteMissingEndpoint(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-test"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("endpoint is required"))
	Expect(result).To(BeNil())
}

func TestExecuteMissingAPIKey(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: "https://owui.example.com"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("api_key is required"))
	Expect(result).To(BeNil())
}

func TestExecuteMissingPrompt(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: "https://owui.example.com"},
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-test"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("prompt is required"))
	Expect(result).To(BeNil())
}

func TestExecuteMissingModel(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: "https://owui.example.com"},
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("model is required"))
	Expect(result).To(BeNil())
}

func TestExecuteChatCompletion(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bearer auth, POST, and the Open WebUI chat path derived from a base URL.
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer sk-test-key"))
		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.URL.Path).To(Equal("/api/chat/completions"))

		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		Expect(reqBody["model"]).To(Equal("llama3.1:8b"))

		messages := reqBody["messages"].([]interface{})
		Expect(len(messages)).To(Equal(2)) // system + user

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.1:8b", "Hello from Open WebUI!", "stop"))
	}))
	defer server.Close()

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: server.URL},
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3.1:8b"},
		{Name: "system_prompt", Type: core.ConnectionTypeText, Value: "You are helpful."},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hi there"},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result).To(Not(BeNil()))
	Expect(result["response"]).To(Equal("Hello from Open WebUI!"))
	Expect(result["model"]).To(Equal("llama3.1:8b"))
	Expect(result["total_tokens"]).To(Equal(int64(30)))
	Expect(result["should_respond"]).To(Equal(true))
	Expect(result["success"]).To(Equal(true))
}

func TestExecuteFullEndpointUsedVerbatim(t *testing.T) {
	RegisterTestingT(t)

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3", "ok", "stop"))
	}))
	defer server.Close()

	inputs := []*core.Connection{
		// Endpoint already ends in /chat/completions → must be used as-is.
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: server.URL + "/v1/chat/completions"},
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hello"},
	}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(gotPath).To(Equal("/v1/chat/completions"))
}

func TestExecuteConversationHistory(t *testing.T) {
	RegisterTestingT(t)

	var capturedMessages []interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		capturedMessages = reqBody["messages"].([]interface{})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3", "answer", "stop"))
	}))
	defer server.Close()

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: server.URL},
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "And after that?"},
		{Name: "conversation_history", Type: core.ConnectionTypeObject, Value: []map[string]interface{}{
			{"role": "user", "content": "First question"},
			{"role": "assistant", "content": "First answer"},
		}},
	}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	// history (2) + new user turn (1); no system prompt supplied.
	Expect(len(capturedMessages)).To(Equal(3))
	last := capturedMessages[2].(map[string]interface{})
	Expect(last["role"]).To(Equal("user"))
	Expect(last["content"]).To(Equal("And after that?"))
}

func TestExecuteOmitsSamplingParamsWhenUnset(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3", "ok", "stop"))
	}))
	defer server.Close()

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: server.URL},
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hello"},
	}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	// Neither field should be present — local models may reject them.
	_, hasMax := reqBody["max_tokens"]
	_, hasTemp := reqBody["temperature"]
	Expect(hasMax).To(BeFalse())
	Expect(hasTemp).To(BeFalse())
}

func TestExecuteSendsSamplingParamsWhenSet(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3", "ok", "stop"))
	}))
	defer server.Close()

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: server.URL},
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hello"},
		{Name: "max_tokens", Type: core.ConnectionTypeInteger, Value: float64(256)},
		{Name: "temperature", Type: core.ConnectionTypeString, Value: "0.2"},
	}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(reqBody["max_tokens"]).To(Equal(float64(256)))
	Expect(reqBody["temperature"]).To(Equal(0.2))
}

func TestExecuteToolCalls(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		Expect(reqBody["tools"]).To(Not(BeNil()))

		resp := map[string]interface{}{
			"id":    "chatcmpl-tool",
			"model": "llama3",
			"usage": map[string]interface{}{"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10},
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

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: server.URL},
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3"},
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
	// Conversation state is carried forward for the engine's next turn.
	Expect(result[core.ToolConversationStateKey]).To(Not(BeNil()))
}

func TestExecuteNoResponseSentinel(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3", "[NO_RESPONSE]", "stop"))
	}))
	defer server.Close()

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: server.URL},
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-test-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3"},
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
				"message": "Invalid API key",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: server.URL},
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "sk-bad-key"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hello"},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Invalid API key"))
	Expect(result).To(BeNil())
}
