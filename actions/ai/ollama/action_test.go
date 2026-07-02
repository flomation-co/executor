package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// chatResponse is a minimal native /api/chat response body.
func chatResponse(model, content string) map[string]interface{} {
	return map[string]interface{}{
		"model": model,
		"message": map[string]interface{}{
			"role":    "assistant",
			"content": content,
		},
		"done":              true,
		"done_reason":       "stop",
		"prompt_eval_count": 10,
		"eval_count":        20,
	}
}

func baseInputs(endpoint string) []*core.Connection {
	return []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3.2"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hello"},
	}
}

func TestChatURLNormalisation(t *testing.T) {
	RegisterTestingT(t)

	for input, want := range map[string]string{
		"http://localhost:11434":           "http://localhost:11434/api/chat",
		"http://localhost:11434/":          "http://localhost:11434/api/chat",
		"http://localhost:11434/api":       "http://localhost:11434/api/chat",
		"http://localhost:11434/api/":      "http://localhost:11434/api/chat",
		"http://localhost:11434/api/chat":  "http://localhost:11434/api/chat",
		"http://localhost:11434/api/chat/": "http://localhost:11434/api/chat",
		"  http://host:11434/  ":           "http://host:11434/api/chat",
	} {
		Expect(chatURL(input)).To(Equal(want), "input: %q", input)
	}
}

func TestExecuteMissingEndpoint(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3.2"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("endpoint is required"))
	Expect(result).To(BeNil())
}

func TestExecuteMissingModel(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: "http://localhost:11434"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("model is required"))
	Expect(result).To(BeNil())
}

func TestExecuteMissingPrompt(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: "http://localhost:11434"},
		{Name: "model", Type: core.ConnectionTypeString, Value: "llama3.2"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("prompt is required"))
	Expect(result).To(BeNil())
}

func TestExecuteBasicChat(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.URL.Path).To(Equal("/api/chat"))
		Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))
		// No api_key input — no Authorization header.
		Expect(r.Header.Get("Authorization")).To(Equal(""))

		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.2", "Hello from Ollama!"))
	}))
	defer server.Close()

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "system_prompt", Type: core.ConnectionTypeText, Value: "You are helpful."},
	)

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result).To(Not(BeNil()))
	Expect(result["response"]).To(Equal("Hello from Ollama!"))
	Expect(result["thinking"]).To(Equal(""))
	Expect(result["model"]).To(Equal("llama3.2"))
	Expect(result["prompt_tokens"]).To(Equal(int64(10)))
	Expect(result["completion_tokens"]).To(Equal(int64(20)))
	Expect(result["total_tokens"]).To(Equal(int64(30)))
	Expect(result["should_respond"]).To(Equal(true))
	Expect(result["success"]).To(Equal(true))

	// Streaming must be explicitly disabled — /api/chat streams by default.
	Expect(reqBody["stream"]).To(Equal(false))
	// No options were set, so no options object is sent at all.
	Expect(reqBody).To(Not(HaveKey("options")))
	Expect(reqBody).To(Not(HaveKey("format")))
	Expect(reqBody).To(Not(HaveKey("think")))
	Expect(reqBody).To(Not(HaveKey("keep_alive")))

	messages := reqBody["messages"].([]interface{})
	Expect(len(messages)).To(Equal(2)) // system + user
	system := messages[0].(map[string]interface{})
	Expect(system["role"]).To(Equal("system"))
	Expect(system["content"]).To(ContainSubstring("You are helpful."))
	user := messages[1].(map[string]interface{})
	Expect(user["role"]).To(Equal("user"))
	Expect(user["content"]).To(Equal("Hello"))
	Expect(user).To(Not(HaveKey("images")))
}

func TestExecuteBearerAuthWhenKeySet(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer proxy-token"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.2", "ok"))
	}))
	defer server.Close()

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "api_key", Type: core.ConnectionTypeString, Value: "proxy-token"},
	)

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
}

func TestExecuteOptionsMapping(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.2", "ok"))
	}))
	defer server.Close()

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "max_tokens", Type: core.ConnectionTypeInteger, Value: float64(512)},
		&core.Connection{Name: "temperature", Type: core.ConnectionTypeString, Value: "0.4"},
		&core.Connection{Name: "top_p", Type: core.ConnectionTypeString, Value: "0.9"},
		&core.Connection{Name: "min_p", Type: core.ConnectionTypeString, Value: "0.05"},
		&core.Connection{Name: "top_k", Type: core.ConnectionTypeInteger, Value: float64(40)},
		&core.Connection{Name: "seed", Type: core.ConnectionTypeInteger, Value: float64(42)},
		&core.Connection{Name: "num_ctx", Type: core.ConnectionTypeInteger, Value: float64(8192)},
		&core.Connection{Name: "repeat_penalty", Type: core.ConnectionTypeString, Value: "1.1"},
		&core.Connection{Name: "repeat_last_n", Type: core.ConnectionTypeInteger, Value: float64(64)},
		&core.Connection{Name: "frequency_penalty", Type: core.ConnectionTypeString, Value: "0.5"},
		&core.Connection{Name: "presence_penalty", Type: core.ConnectionTypeString, Value: "0.6"},
		&core.Connection{Name: "num_batch", Type: core.ConnectionTypeInteger, Value: float64(256)},
		&core.Connection{Name: "num_gpu", Type: core.ConnectionTypeInteger, Value: float64(-1)},
		&core.Connection{Name: "main_gpu", Type: core.ConnectionTypeInteger, Value: float64(0)},
		&core.Connection{Name: "num_thread", Type: core.ConnectionTypeInteger, Value: float64(8)},
		&core.Connection{Name: "low_vram", Type: core.ConnectionTypeBoolean, Value: true},
		&core.Connection{Name: "use_mlock", Type: core.ConnectionTypeBoolean, Value: true},
		&core.Connection{Name: "use_mmap", Type: core.ConnectionTypeBoolean, Value: false},
		&core.Connection{Name: "penalize_newline", Type: core.ConnectionTypeBoolean, Value: false},
		&core.Connection{Name: "stop", Type: core.ConnectionTypeString, Value: "###, END,  "},
		&core.Connection{Name: "keep_alive", Type: core.ConnectionTypeString, Value: "10m"},
		&core.Connection{Name: "response_format", Type: core.ConnectionTypeString, Value: "json"},
		&core.Connection{Name: "think", Type: core.ConnectionTypeString, Value: "on"},
	)

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())

	// format, keep_alive and think are top-level fields, never nested in
	// options (where the server would silently ignore them).
	Expect(reqBody["format"]).To(Equal("json"))
	Expect(reqBody["keep_alive"]).To(Equal("10m"))
	Expect(reqBody["think"]).To(Equal(true))

	options := reqBody["options"].(map[string]interface{})
	Expect(options["num_predict"]).To(Equal(float64(512)))
	Expect(options["temperature"]).To(Equal(0.4))
	Expect(options["top_p"]).To(Equal(0.9))
	Expect(options["min_p"]).To(Equal(0.05))
	Expect(options["top_k"]).To(Equal(float64(40)))
	Expect(options["seed"]).To(Equal(float64(42)))
	Expect(options["num_ctx"]).To(Equal(float64(8192)))
	Expect(options["repeat_penalty"]).To(Equal(1.1))
	Expect(options["repeat_last_n"]).To(Equal(float64(64)))
	Expect(options["frequency_penalty"]).To(Equal(0.5))
	Expect(options["presence_penalty"]).To(Equal(0.6))
	Expect(options["num_batch"]).To(Equal(float64(256)))
	Expect(options["num_gpu"]).To(Equal(float64(-1)))
	Expect(options["main_gpu"]).To(Equal(float64(0)))
	Expect(options["num_thread"]).To(Equal(float64(8)))
	Expect(options["low_vram"]).To(Equal(true))
	Expect(options["use_mlock"]).To(Equal(true))
	Expect(options["use_mmap"]).To(Equal(false))
	Expect(options["penalize_newline"]).To(Equal(false))
	Expect(options["stop"]).To(Equal([]interface{}{"###", "END"}))
	Expect(options).To(Not(HaveKey("format")))
	Expect(options).To(Not(HaveKey("keep_alive")))
	Expect(options).To(Not(HaveKey("think")))
}

// TestExecuteKeepAliveSpellings pins that bare-number keep_alive values
// are sent as JSON numbers: the server parses string values with Go's
// ParseDuration, which rejects "-1"/"300" and would fail the whole request
// with HTTP 400.
func TestExecuteKeepAliveSpellings(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.2", "ok"))
	}))
	defer server.Close()

	for value, want := range map[string]interface{}{
		"-1":    float64(-1),
		"300":   float64(300),
		"0":     float64(0),
		"1h30m": "1h30m",
	} {
		reqBody = nil
		_, err := Execute(&core.Flow{}, nil, append(baseInputs(server.URL),
			&core.Connection{Name: "keep_alive", Type: core.ConnectionTypeString, Value: value},
		))
		Expect(err).To(BeNil())
		Expect(reqBody["keep_alive"]).To(Equal(want), "keep_alive value: %q", value)
	}
}

func TestExecuteMalformedFloatsOmitted(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.2", "ok"))
	}))
	defer server.Close()

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "temperature", Type: core.ConnectionTypeString, Value: "warm"},
		&core.Connection{Name: "top_p", Type: core.ConnectionTypeString, Value: ""},
	)

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	// Malformed and blank values are omitted so model defaults apply.
	Expect(reqBody).To(Not(HaveKey("options")))
}

func TestExecuteThinkTriState(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("qwen3", "ok"))
	}))
	defer server.Close()

	for value, expectation := range map[string]interface{}{
		"default": nil,
		"":        nil,
		"on":      true,
		"off":     false,
		// Reasoning-effort levels are the string form of think that
		// gpt-oss-family models accept in place of true.
		"High":    "high",
		"garbage": nil,
	} {
		reqBody = nil
		inputs := append(baseInputs(server.URL),
			&core.Connection{Name: "think", Type: core.ConnectionTypeString, Value: value},
		)
		_, err := Execute(&core.Flow{}, nil, inputs)
		Expect(err).To(BeNil())
		if expectation == nil {
			Expect(reqBody).To(Not(HaveKey("think")), "think value: %q", value)
		} else {
			Expect(reqBody["think"]).To(Equal(expectation), "think value: %q", value)
		}
	}
}

func TestExecuteResponseFormatVariants(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.2", "ok"))
	}))
	defer server.Close()

	// json_object (the OpenAI-family spelling) maps to native "json";
	// unknown text warns and falls back to text mode.
	for value, expectation := range map[string]interface{}{
		"text":        nil,
		"json":        "json",
		"JSON_OBJECT": "json",
		"xml":         nil,
	} {
		reqBody = nil
		inputs := append(baseInputs(server.URL),
			&core.Connection{Name: "response_format", Type: core.ConnectionTypeString, Value: value},
		)
		_, err := Execute(&core.Flow{}, nil, inputs)
		Expect(err).To(BeNil())
		if expectation == nil {
			Expect(reqBody).To(Not(HaveKey("format")), "format value: %q", value)
		} else {
			Expect(reqBody["format"]).To(Equal(expectation), "format value: %q", value)
		}
	}

	// A raw JSON-schema object is forwarded for structured outputs.
	reqBody = nil
	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "response_format", Type: core.ConnectionTypeString,
			Value: `{"type":"object","properties":{"answer":{"type":"string"}}}`},
	)
	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	schema := reqBody["format"].(map[string]interface{})
	Expect(schema["type"]).To(Equal("object"))
}

func TestExecuteConversationHistory(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.2", "ok"))
	}))
	defer server.Close()

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "conversation_history", Type: core.ConnectionTypeObject,
			Value: `[{"role":"user","content":"first"},{"role":"assistant","content":"second"}]`},
	)

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())

	// The blob-token instructions always yield a system message, matching
	// the other AI actions.
	messages := reqBody["messages"].([]interface{})
	Expect(len(messages)).To(Equal(4)) // system + history x2 + user prompt
	Expect(messages[0].(map[string]interface{})["role"]).To(Equal("system"))
	Expect(messages[1].(map[string]interface{})["content"]).To(Equal("first"))
	Expect(messages[2].(map[string]interface{})["role"]).To(Equal("assistant"))
	Expect(messages[3].(map[string]interface{})["content"]).To(Equal("Hello"))
}

func TestExecuteToolCallFirstLeg(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// OpenAI-style manual tool definitions pass through verbatim.
		tools := reqBody["tools"].([]interface{})
		Expect(len(tools)).To(Equal(1))
		fn := tools[0].(map[string]interface{})["function"].(map[string]interface{})
		Expect(fn["name"]).To(Equal("get_weather"))

		w.Header().Set("Content-Type", "application/json")
		// Native tool_calls: no id, arguments is a JSON object.
		json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "llama3.2",
			"message": map[string]interface{}{
				"role":     "assistant",
				"content":  "Checking the weather now.",
				"thinking": "The user wants weather; call the tool.",
				"tool_calls": []map[string]interface{}{
					{"function": map[string]interface{}{
						"name":      "get_weather",
						"arguments": map[string]interface{}{"city": "London"},
					}},
					{"function": map[string]interface{}{
						"name":      "get_time",
						"arguments": map[string]interface{}{},
					}},
				},
			},
			"done":              true,
			"done_reason":       "stop",
			"prompt_eval_count": 15,
			"eval_count":        25,
		})
	}))
	defer server.Close()

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "tool_definitions", Type: core.ConnectionTypeText,
			Value: `[{"type":"function","function":{"name":"get_weather","description":"Get weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}]`},
	)

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["stop_reason"]).To(Equal("tool_calls"))
	Expect(result["tool_calls_count"]).To(Equal(2))
	Expect(result[core.IntermediateTextKey]).To(Equal("Checking the weather now."))

	requests := result[core.ToolRequestsKey].([]core.ToolRequest)
	Expect(len(requests)).To(Equal(2))
	Expect(requests[0].Name).To(Equal("get_weather"))
	Expect(requests[0].ID).To(Not(BeEmpty()))
	Expect(requests[0].Input).To(Equal(map[string]interface{}{"city": "London"}))
	// Zero-parameter tool: input must be an empty non-nil map.
	Expect(requests[1].Input).To(Equal(map[string]interface{}{}))
	Expect(requests[0].ID).To(Not(Equal(requests[1].ID)))

	// Conversation state carries the assistant message with the
	// synthesised ids so the re-entry leg can map results to names.
	state := result[core.ToolConversationStateKey].([]interface{})
	assistant := state[len(state)-1].(map[string]interface{})
	Expect(assistant["role"]).To(Equal("assistant"))
	// The thinking trace must survive into the echoed message — thinking
	// models re-read it on the re-entry leg.
	Expect(assistant["thinking"]).To(Equal("The user wants weather; call the tool."))
	calls := assistant["tool_calls"].([]interface{})
	Expect(len(calls)).To(Equal(2))
	first := calls[0].(map[string]interface{})
	Expect(first["id"]).To(Equal(requests[0].ID))
}

func TestExecuteAnthropicStyleToolDefsNormalised(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.2", "ok"))
	}))
	defer server.Close()

	// The engine's Tools-handle auto-discovery injects Anthropic-style
	// definitions; they must be converted to Ollama's function schema.
	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "tool_definitions", Type: core.ConnectionTypeText,
			Value: `[{"name":"slack_send","description":"Send a Slack message","input_schema":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}}]`},
	)

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())

	tools := reqBody["tools"].([]interface{})
	Expect(len(tools)).To(Equal(1))
	tool := tools[0].(map[string]interface{})
	Expect(tool["type"]).To(Equal("function"))
	fn := tool["function"].(map[string]interface{})
	Expect(fn["name"]).To(Equal("slack_send"))
	Expect(fn["description"]).To(Equal("Send a Slack message"))
	params := fn["parameters"].(map[string]interface{})
	Expect(params["required"]).To(Equal([]interface{}{"text"}))
}

func TestExecuteToolResultReinvocation(t *testing.T) {
	RegisterTestingT(t)

	var capturedMessages []interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		capturedMessages = reqBody["messages"].([]interface{})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.2", "It is sunny."))
	}))
	defer server.Close()

	flow := &core.Flow{}
	// Prior turn's state as this action stores it: synthesised ids on the
	// assistant tool_calls, arguments as objects.
	flow.SetVariable(core.ToolConversationStateKey, []interface{}{
		map[string]interface{}{"role": "user", "content": "What's the weather?"},
		map[string]interface{}{
			"role":    "assistant",
			"content": "",
			"tool_calls": []interface{}{
				map[string]interface{}{
					"id":   "call_1_0",
					"type": "function",
					"function": map[string]interface{}{
						"name":      "get_weather",
						"arguments": map[string]interface{}{"city": "London"},
					},
				},
			},
		},
	})
	flow.SetVariable(core.ToolResultsKey, []core.ToolResult{
		{ToolUseID: "call_1_0", Content: "sunny, 25C"},
	})

	result, err := Execute(flow, nil, baseInputs(server.URL))
	Expect(err).To(BeNil())
	Expect(result["response"]).To(Equal("It is sunny."))

	// The tool result is echoed as a `tool` message carrying tool_name —
	// Ollama matches results by name, not id.
	Expect(len(capturedMessages)).To(Equal(3)) // user + assistant + tool
	toolMsg := capturedMessages[2].(map[string]interface{})
	Expect(toolMsg["role"]).To(Equal("tool"))
	Expect(toolMsg["tool_name"]).To(Equal("get_weather"))
	Expect(toolMsg["content"]).To(Equal("sunny, 25C"))
	Expect(toolMsg).To(Not(HaveKey("tool_call_id")))
}

func TestExecuteThinkingOutput(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "qwen3",
			"message": map[string]interface{}{
				"role":     "assistant",
				"content":  "4",
				"thinking": "2+2 is elementary arithmetic.",
			},
			"done":              true,
			"done_reason":       "stop",
			"prompt_eval_count": 5,
			"eval_count":        50,
		})
	}))
	defer server.Close()

	result, err := Execute(&core.Flow{}, nil, baseInputs(server.URL))
	Expect(err).To(BeNil())
	Expect(result["response"]).To(Equal("4"))
	Expect(result["thinking"]).To(Equal("2+2 is elementary arithmetic."))
}

func TestExecuteAPIError(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": `model "missing" not found, try pulling it first`,
		})
	}))
	defer server.Close()

	result, err := Execute(&core.Flow{}, nil, baseInputs(server.URL))
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("Ollama API error (404)"))
	Expect(err.Error()).To(ContainSubstring("not found, try pulling it"))
	Expect(result).To(BeNil())
}

func TestExecuteNoResponseSentinel(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("llama3.2", "[NO_RESPONSE]"))
	}))
	defer server.Close()

	result, err := Execute(&core.Flow{}, nil, baseInputs(server.URL))
	Expect(err).To(BeNil())
	Expect(result["should_respond"]).To(Equal(false))
	Expect(result["response"]).To(Equal(""))
}

func TestExecuteModelFallbackWhenAbsent(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]interface{}{"role": "assistant", "content": "ok"},
			"done":    true,
		})
	}))
	defer server.Close()

	result, err := Execute(&core.Flow{}, nil, baseInputs(server.URL))
	Expect(err).To(BeNil())
	Expect(result["model"]).To(Equal("llama3.2"))
	Expect(result["total_tokens"]).To(Equal(int64(0)))
}

func TestBuildUserMessageWithImages(t *testing.T) {
	RegisterTestingT(t)

	fetcher := fakeFetcher{
		"flo:blob:0123456789abcdef0123456789abcdef": []byte{0xFF, 0xD8, 0xFF, 0xE0},
	}
	prompt := "Describe this. [attached: photo.jpg (image/jpeg, 4 B) → flo:blob:0123456789abcdef0123456789abcdef]"

	msg := buildUserMessage(prompt, fetcher)
	Expect(msg["role"]).To(Equal("user"))
	Expect(msg["content"]).To(ContainSubstring("Describe this."))
	images := msg["images"].([]string)
	Expect(len(images)).To(Equal(1))
	Expect(images[0]).To(Equal("/9j/4A==")) // base64 of the JPEG magic bytes
}

func TestBuildUserMessageWithoutImages(t *testing.T) {
	RegisterTestingT(t)

	msg := buildUserMessage("Just text", fakeFetcher{})
	Expect(msg["content"]).To(Equal("Just text"))
	Expect(msg).To(Not(HaveKey("images")))
}

type fakeFetcher map[string][]byte

func (f fakeFetcher) Get(token string) ([]byte, error) {
	if b, ok := f[token]; ok {
		return b, nil
	}
	return nil, http.ErrMissingFile
}
