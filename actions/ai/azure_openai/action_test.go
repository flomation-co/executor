package azure_openai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// chatResponse is a minimal Azure OpenAI Chat Completions response body.
// Azure mirrors the OpenAI shape but returns no provider and no usage.cost —
// the model field carries the underlying model, not the deployment name.
// content is interface{} so a test can serve the null Azure returns when the
// completion-side filter withholds the text.
func chatResponse(model string, content interface{}, finishReason string) map[string]interface{} {
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

// baseInputs wires the httptest server in through the Custom Endpoint input —
// the action's own test seam: endpoint wins over resource_name, so no URL
// rewriting or package-var swapping is needed.
func baseInputs(serverURL string) []*core.Connection {
	return []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "azure-test-key"},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: serverURL},
		{Name: "deployment", Type: core.ConnectionTypeString, Value: "gpt-4o"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Hi there"},
	}
}

func TestExecuteMissingAPIKey(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "deployment", Type: core.ConnectionTypeString, Value: "gpt-4o"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("api_key is required"))
	Expect(result).To(BeNil())
}

func TestExecuteMissingPrompt(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "azure-test-key"},
		{Name: "deployment", Type: core.ConnectionTypeString, Value: "gpt-4o"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("prompt is required"))
	Expect(result).To(BeNil())
}

func TestExecuteMissingDeployment(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "azure-test-key"},
		{Name: "resource_name", Type: core.ConnectionTypeString, Value: "my-resource"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("deployment is required"))
	Expect(result).To(BeNil())
}

func TestExecuteMissingResourceNameAndEndpoint(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: "azure-test-key"},
		{Name: "deployment", Type: core.ConnectionTypeString, Value: "gpt-4o"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("either resource_name or endpoint is required"))
}

// The resource name is interpolated into a hostname, so its charset is the
// security boundary: "evil.example.com/v1?x=" must never become the host the
// operator's key is sent to.
func TestExecuteResourceNameCharsetValidated(t *testing.T) {
	RegisterTestingT(t)

	for _, bad := range []string{"my.resource", "my resource", "res/../../x", "res?a=b", "res#frag", "münchen"} {
		_, err := Execute(&core.Flow{}, nil, []*core.Connection{
			{Name: "api_key", Type: core.ConnectionTypeString, Value: "azure-test-key"},
			{Name: "resource_name", Type: core.ConnectionTypeString, Value: bad},
			{Name: "deployment", Type: core.ConnectionTypeString, Value: "gpt-4o"},
			{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
		})
		Expect(err).To(HaveOccurred(), "resource_name %q must be rejected", bad)
		Expect(err.Error()).To(ContainSubstring("invalid characters"))
	}
}

func TestExecuteChatCompletion(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The Azure deltas: api-key header (never Bearer), the deployment in
		// the URL path, and the mandatory api-version query param.
		Expect(r.Header.Get("api-key")).To(Equal("azure-test-key"))
		Expect(r.Header.Get("Authorization")).To(Equal(""))
		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.URL.Path).To(Equal("/openai/deployments/gpt-4o/chat/completions"))
		Expect(r.URL.Query().Get("api-version")).To(Equal("2024-10-21"))

		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		// The deployment in the URL decides the model; no model body field.
		Expect(reqBody).To(Not(HaveKey("model")))

		messages := reqBody["messages"].([]interface{})
		Expect(len(messages)).To(Equal(2)) // system + user

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o-2024-08-06", "Hello from Azure!", "stop"))
	}))
	defer server.Close()

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "system_prompt", Type: core.ConnectionTypeText, Value: "You are helpful."})

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result).To(Not(BeNil()))
	Expect(result["response"]).To(Equal("Hello from Azure!"))
	Expect(result["model"]).To(Equal("gpt-4o-2024-08-06"))
	Expect(result["total_tokens"]).To(Equal(int64(30)))
	Expect(result["should_respond"]).To(Equal(true))
	Expect(result["success"]).To(Equal(true))

	// Azure returns neither a winning provider nor a per-request cost, so
	// the openrouter outputs of those names are deliberately absent.
	Expect(result).To(Not(HaveKey("provider")))
	Expect(result).To(Not(HaveKey("cost")))
}

func TestExecuteAPIVersionOverride(t *testing.T) {
	RegisterTestingT(t)

	var gotVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.URL.Query().Get("api-version")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o", "ok", "stop"))
	}))
	defer server.Close()

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "api_version", Type: core.ConnectionTypeString, Value: "2025-03-01-preview"})

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(gotVersion).To(Equal("2025-03-01-preview"))
}

// A deployment name is customer-chosen text going into a URL path segment, so
// it must be segment-encoded rather than trusted.
func TestExecuteDeploymentPathEscaped(t *testing.T) {
	RegisterTestingT(t)

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o", "ok", "stop"))
	}))
	defer server.Close()

	inputs := baseInputs(server.URL)
	inputs[2] = &core.Connection{Name: "deployment", Type: core.ConnectionTypeString, Value: "my dep/x"}

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(gotPath).To(Equal("/openai/deployments/my%20dep%2Fx/chat/completions"))
}

func TestExecuteResponseFormatJSONObject(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o", "{}", "stop"))
	}))
	defer server.Close()

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "response_format", Type: core.ConnectionTypeString, Value: "json_object"})

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(reqBody["response_format"]).To(Equal(map[string]interface{}{"type": "json_object"}))
}

func TestExecuteResponseFormatJSONSchema(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o", `{"answer":"42"}`, "stop"))
	}))
	defer server.Close()

	schema := `{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`
	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "response_format", Type: core.ConnectionTypeString, Value: "json_schema"},
		&core.Connection{Name: "json_schema", Type: core.ConnectionTypeText, Value: schema})

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())

	rf, ok := reqBody["response_format"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(rf["type"]).To(Equal("json_schema"))
	js, ok := rf["json_schema"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(js["name"]).To(Equal("response"))
	Expect(js["strict"]).To(Equal(true))
	inner, ok := js["schema"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(inner["type"]).To(Equal("object"))
}

func TestExecuteJSONSchemaMissingOrInvalid(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a broken schema must fail before any request is made")
	}))
	defer server.Close()

	// Selected json_schema mode but left the schema empty.
	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "response_format", Type: core.ConnectionTypeString, Value: "json_schema"})
	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("json_schema is required"))

	// Malformed schema JSON.
	inputs = append(baseInputs(server.URL),
		&core.Connection{Name: "response_format", Type: core.ConnectionTypeString, Value: "json_schema"},
		&core.Connection{Name: "json_schema", Type: core.ConnectionTypeText, Value: "{not json"})
	_, err = Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("not valid JSON"))
}

func TestExecuteSamplingParamsOmittedWhenUnset(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o", "ok", "stop"))
	}))
	defer server.Close()

	_, err := Execute(&core.Flow{}, nil, baseInputs(server.URL))
	Expect(err).To(BeNil())
	Expect(reqBody).To(Not(HaveKey("top_p")))
	Expect(reqBody).To(Not(HaveKey("frequency_penalty")))
	Expect(reqBody).To(Not(HaveKey("presence_penalty")))
	Expect(reqBody).To(Not(HaveKey("response_format")))
	// The always-sent baseline params remain.
	Expect(reqBody["temperature"]).To(Equal(0.7))
	Expect(reqBody["max_tokens"]).To(Equal(float64(2048)))
}

func TestExecuteToolCalls(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		Expect(reqBody["tools"]).To(Not(BeNil()))

		resp := map[string]interface{}{
			"id":    "chatcmpl-tool",
			"model": "gpt-4o-2024-08-06",
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

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "tool_definitions", Type: core.ConnectionTypeText, Value: `[{"type":"function","function":{"name":"web_search","description":"Search","parameters":{"type":"object","properties":{"query":{"type":"string"}}}}}]`})

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

// The second leg of the tool loop: the engine re-invokes Execute with the
// prior conversation state and the executed tool results, and the action must
// append those results as `tool` messages and return the final text answer.
func TestExecuteToolResultReinvocation(t *testing.T) {
	RegisterTestingT(t)

	var capturedMessages []interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		capturedMessages = reqBody["messages"].([]interface{})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o", "It is sunny.", "stop"))
	}))
	defer server.Close()

	flow := &core.Flow{}
	flow.SetVariable(core.ToolConversationStateKey, []interface{}{
		map[string]interface{}{"role": "user", "content": "What's the weather?"},
		map[string]interface{}{
			"role": "assistant",
			"tool_calls": []map[string]interface{}{
				{"id": "call_1", "type": "function", "function": map[string]interface{}{"name": "web_search", "arguments": `{"query":"weather"}`}},
			},
		},
	})
	flow.SetVariable(core.ToolResultsKey, []core.ToolResult{
		{ToolUseID: "call_1", Content: "sunny, 25C"},
	})

	result, err := Execute(flow, nil, baseInputs(server.URL))
	Expect(err).To(BeNil())
	Expect(result["response"]).To(Equal("It is sunny."))
	Expect(result["should_respond"]).To(Equal(true))

	Expect(len(capturedMessages)).To(Equal(3)) // user + assistant + tool
	toolMsg := capturedMessages[2].(map[string]interface{})
	Expect(toolMsg["role"]).To(Equal("tool"))
	Expect(toolMsg["tool_call_id"]).To(Equal("call_1"))
	Expect(toolMsg["content"]).To(Equal("sunny, 25C"))
}

func TestExecuteNoResponseSentinel(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o", "[NO_RESPONSE]", "stop"))
	}))
	defer server.Close()

	result, err := Execute(&core.Flow{}, nil, baseInputs(server.URL))
	Expect(err).To(BeNil())
	Expect(result["should_respond"]).To(Equal(false))
	Expect(result["response"]).To(Equal(""))
}

// A content-filter 400 is Azure refusing the PROMPT — an outcome the flow
// should branch on, not a hard failure that kills the run.
func TestExecuteContentFilterIsASoftError(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "content_filter",
				"message": "The response was filtered due to the prompt triggering Azure OpenAI's content management policy.",
			},
		})
	}))
	defer server.Close()

	result, err := Execute(&core.Flow{}, nil, baseInputs(server.URL))
	Expect(err).To(BeNil(), "content filter must be a soft error, not a run-killing one")
	Expect(result).To(Not(BeNil()))
	Expect(result["success"]).To(Equal(false))
	Expect(result["should_respond"]).To(Equal(false))
	Expect(result["response"]).To(Equal(""))
	Expect(result["error"]).To(ContainSubstring("content filter"))
	Expect(result["error"]).To(ContainSubstring("content management policy"))
}

// The completion half of the filter is an HTTP 200 with finish_reason
// "content_filter" and no content — it must fail the same soft way as the
// prompt half, not report success with an empty response the flow sends on.
func TestExecuteCompletionContentFilterIsASoftError(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o", nil, "content_filter"))
	}))
	defer server.Close()

	result, err := Execute(&core.Flow{}, nil, baseInputs(server.URL))
	Expect(err).To(BeNil(), "content filter must be a soft error, not a run-killing one")
	Expect(result).To(Not(BeNil()))
	Expect(result["success"]).To(Equal(false))
	Expect(result["should_respond"]).To(Equal(false))
	Expect(result["response"]).To(Equal(""))
	Expect(result["error"]).To(ContainSubstring("content filter"))
}

// Azure often returns the text it had emitted before the filter cut in; it is
// the only context the operator gets on this path, so it rides on the error.
func TestExecuteCompletionContentFilterKeepsPartialContent(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o", "Here is how you could", "content_filter"))
	}))
	defer server.Close()

	result, err := Execute(&core.Flow{}, nil, baseInputs(server.URL))
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(false))
	Expect(result["should_respond"]).To(Equal(false))
	Expect(result["response"]).To(Equal(""), "partial filtered text must not be presented as the response")
	Expect(result["error"]).To(ContainSubstring("content filter"))
	Expect(result["error"]).To(ContainSubstring("Here is how you could"))
}

// Every other API error stays hard — and whatever Azure echoes back, the
// operator's key must not appear in the error string.
func TestExecuteAPIErrorHardAndRedacted(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "401",
				"message": "Access denied due to invalid subscription key azure-test-key. Make sure to provide a valid key for an active subscription.",
			},
		})
	}))
	defer server.Close()

	result, err := Execute(&core.Flow{}, nil, baseInputs(server.URL))
	Expect(err).To(HaveOccurred())
	Expect(result).To(BeNil())
	Expect(err.Error()).To(ContainSubstring("Access denied"))
	Expect(err.Error()).To(Not(ContainSubstring("azure-test-key")), "the API key leaked into the error: %s", err.Error())
	Expect(err.Error()).To(ContainSubstring("********"))
}

func TestExecuteInvalidTemperatureFallsBack(t *testing.T) {
	RegisterTestingT(t)

	var reqBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse("gpt-4o", "ok", "stop"))
	}))
	defer server.Close()

	inputs := append(baseInputs(server.URL),
		&core.Connection{Name: "temperature", Type: core.ConnectionTypeString, Value: "not-a-number"})

	_, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(reqBody["temperature"]).To(Equal(0.7))
}

// ---------------------------------------------------------------------------
// Structural invariants
// ---------------------------------------------------------------------------

// TestAuthBlockDoesNotDrift pins the Inputs literal's leading auth/target
// block against the canonical AuthInputs. The manifest generator AST-parses
// the literal array, so the canonical var is documentation — this test is the
// enforcement.
func TestAuthBlockDoesNotDrift(t *testing.T) {
	if len(Inputs) < len(AuthInputs) {
		t.Fatalf("Inputs has %d entries, fewer than the %d-field auth block", len(Inputs), len(AuthInputs))
	}
	for i, want := range AuthInputs {
		if !reflect.DeepEqual(Inputs[i], want) {
			t.Errorf("auth input %d (%q) has drifted from AuthInputs\n got: %+v\nwant: %+v",
				i, want.Name, Inputs[i], want)
		}
	}
}

// No later input may reuse an auth input's name: core.FindConnection returns
// the FIRST match, so a duplicate would silently read the credential.
func TestNoInputShadowsTheAuthBlock(t *testing.T) {
	reserved := map[string]bool{}
	for _, c := range AuthInputs {
		reserved[c.Name] = true
	}
	for _, c := range Inputs[len(AuthInputs):] {
		if reserved[c.Name] {
			t.Errorf("input %q shadows the auth input of the same name", c.Name)
		}
	}
}

// The outputs contract: openrouter's shape minus provider and cost (Azure
// returns neither), everything else identical so agent flows can swap the
// provider without rewiring.
func TestOutputsDropProviderAndCost(t *testing.T) {
	RegisterTestingT(t)

	names := map[string]bool{}
	for _, o := range Outputs {
		names[o.Name] = true
	}
	Expect(names).To(Not(HaveKey("provider")))
	Expect(names).To(Not(HaveKey("cost")))
	for _, required := range []string{
		"response", "model", "prompt_tokens", "completion_tokens",
		"total_tokens", "should_respond", "tool_calls_count", "success", "error",
	} {
		Expect(names).To(HaveKey(required))
	}
}

func TestActionMetadata(t *testing.T) {
	RegisterTestingT(t)

	Expect(Name).To(Equal("Azure OpenAI Prompt"))
	Expect(Icon).To(Equal("brain+cloud"))
	Expect(Type).To(Equal(core.ActionTypeAction))
}
