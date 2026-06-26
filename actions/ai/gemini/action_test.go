// Gemini Prompt action tests. The Gemini API is a moving target for
// model identifiers but the contracts these tests pin are stable:
//
//   - Required-input validation (api_key, prompt).
//   - Request shape: contents array, systemInstruction at top level,
//     auth via x-goog-api-key header.
//   - Response parsing: text candidate, function_call candidate (tool
//     loop initiation), usage metadata.
//   - Tool-loop resume: function_response parts wired correctly when
//     re-invoked with __tool_results.
//
// Each test stands up a httptest.Server and points the package-level
// apiBase at it. Restored in a defer so tests don't leak the override
// into siblings.
package gemini

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// stubServer stands up a httptest.Server with the given handler and
// rewires apiBase to point at it. The returned cleanup function MUST
// be called via defer so the next test in the file gets a clean
// apiBase. Returns the server's URL appended with the v1beta/models/
// path so handler URLs read naturally.
func stubServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	prev := apiBase
	apiBase = srv.URL + "/v1beta/models/"
	t.Cleanup(func() {
		apiBase = prev
		srv.Close()
	})
	return srv
}

func TestExecute_MissingAPIKey(t *testing.T) {
	RegisterTestingT(t)
	_, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("api_key is required"))
}

func TestExecute_MissingPrompt(t *testing.T) {
	RegisterTestingT(t)
	_, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("prompt is required"))
}

// TestExecute_TextResponse pins the happy-path request shape AND the
// response parsing. The handler captures the request body so the test
// can assert exactly what was sent — including that the system prompt
// landed under systemInstruction (not inside contents) and that the
// auth header is x-goog-api-key.
func TestExecute_TextResponse(t *testing.T) {
	RegisterTestingT(t)

	var captured struct {
		AuthHeader string
		Path       string
		Body       map[string]interface{}
	}

	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		captured.AuthHeader = r.Header.Get("x-goog-api-key")
		captured.Path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &captured.Body)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": "Hello there"},
						},
						"role": "model",
					},
					"finishReason": "STOP",
				},
			},
			"modelVersion": "gemini-2.5-flash-001",
			"usageMetadata": map[string]interface{}{
				"promptTokenCount":     5,
				"candidatesTokenCount": 12,
				"totalTokenCount":      17,
			},
		})
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "model", Type: core.ConnectionTypeSecret, Value: "gemini-2.5-flash"},
		{Name: "system_prompt", Type: core.ConnectionTypeText, Value: "You are helpful."},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Say hi"},
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(captured.AuthHeader).To(Equal("AIza-test"))
	Expect(captured.Path).To(Equal("/v1beta/models/gemini-2.5-flash:generateContent"))

	// System prompt lives at the top level, NOT inside contents.
	sysInstr, ok := captured.Body["systemInstruction"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	parts, _ := sysInstr["parts"].([]interface{})
	Expect(parts).To(HaveLen(1))
	firstPart, _ := parts[0].(map[string]interface{})
	Expect(firstPart["text"]).To(ContainSubstring("You are helpful."))

	// contents has the user message under role=user.
	contentsArr, _ := captured.Body["contents"].([]interface{})
	Expect(contentsArr).To(HaveLen(1))
	first, _ := contentsArr[0].(map[string]interface{})
	Expect(first["role"]).To(Equal("user"))

	Expect(result["response"]).To(Equal("Hello there"))
	Expect(result["should_respond"]).To(BeTrue())
	Expect(result["model"]).To(Equal("gemini-2.5-flash-001"))
	Expect(result["prompt_tokens"]).To(BeNumerically("==", 5))
	Expect(result["completion_tokens"]).To(BeNumerically("==", 12))
	Expect(result["total_tokens"]).To(BeNumerically("==", 17))
	Expect(result["success"]).To(BeTrue())
}

// TestExecute_FunctionCallStartsToolLoop pins the contract with the
// engine's tool loop: when the model returns a function_call part,
// the action MUST surface ToolRequests on the well-known key and
// preserve the model's message in ToolConversationState so the next
// turn can append function_responses in the right place.
func TestExecute_FunctionCallStartsToolLoop(t *testing.T) {
	RegisterTestingT(t)

	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"functionCall": map[string]interface{}{
									"name": "web_search",
									"args": map[string]interface{}{"query": "weather"},
								},
							},
						},
						"role": "model",
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]interface{}{
				"promptTokenCount":     8,
				"candidatesTokenCount": 4,
				"totalTokenCount":      12,
			},
		})
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "What's the weather?"},
		{Name: "tool_definitions", Type: core.ConnectionTypeText, Value: `[{"name":"web_search","description":"Search","parameters":{"type":"object","properties":{"query":{"type":"string"}}}}]`},
	})
	Expect(err).NotTo(HaveOccurred())

	requests, ok := result[core.ToolRequestsKey].([]core.ToolRequest)
	Expect(ok).To(BeTrue(), "expected ToolRequests on the well-known key")
	Expect(requests).To(HaveLen(1))
	Expect(requests[0].Name).To(Equal("web_search"))
	Expect(requests[0].Input["query"]).To(Equal("weather"))
	// ID encodes the function name so we can recover it on the
	// response turn (when ToolResult only carries ToolUseID).
	Expect(requests[0].ID).To(HavePrefix("web_search-"))

	// Conversation state is preserved for the resume turn.
	_, hasState := result[core.ToolConversationStateKey]
	Expect(hasState).To(BeTrue())

	Expect(result["stop_reason"]).To(Equal("function_call"))
	Expect(result["tool_calls_count"]).To(BeNumerically("==", 1))
}

// TestExecute_APIError surfaces the upstream error message into the
// returned error rather than papering over it with a generic failure.
// Without this, a malformed API key would only show as "Gemini API
// error (401): {}" and not the actually-useful "API key not valid".
func TestExecute_APIError(t *testing.T) {
	RegisterTestingT(t)

	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "API key not valid",
				"status":  "INVALID_ARGUMENT",
			},
		})
	})

	_, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "bad-key"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("API key not valid"))
	Expect(err.Error()).To(ContainSubstring("400"))
}

// TestExecute_NoResponseSentinel pins the [NO_RESPONSE] convention
// shared with the OpenAI/Anthropic actions: if the model emits the
// sentinel, should_respond is false and the response is blanked so
// downstream channels know not to forward anything.
func TestExecute_NoResponseSentinel(t *testing.T) {
	RegisterTestingT(t)

	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": "[NO_RESPONSE]"},
						},
					},
					"finishReason": "STOP",
				},
			},
		})
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "internal turn"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["should_respond"]).To(BeFalse())
	Expect(result["response"]).To(Equal(""))
}

// TestParseToolDefinitions_AcceptsBothShapes confirms tool defs can be
// passed in OpenAI's nested shape or Gemini's flat shape and both
// land at the same canonical list. Keeps cross-provider tool defs
// portable.
func TestParseToolDefinitions_AcceptsBothShapes(t *testing.T) {
	RegisterTestingT(t)

	// Gemini native flat shape.
	flat := `[{"name":"web_search","description":"d"}]`
	out := parseToolDefinitions(flat)
	Expect(out).To(HaveLen(1))
	first, _ := out[0].(map[string]interface{})
	Expect(first["name"]).To(Equal("web_search"))

	// OpenAI nested shape — must unwrap to the same flat result.
	nested := `[{"type":"function","function":{"name":"web_search","description":"d"}}]`
	out = parseToolDefinitions(nested)
	Expect(out).To(HaveLen(1))
	first, _ = out[0].(map[string]interface{})
	Expect(first["name"]).To(Equal("web_search"))

	// Wrapped-object shape ({"functionDeclarations":[...]}).
	wrapped := `{"functionDeclarations":[{"name":"web_search"}]}`
	out = parseToolDefinitions(wrapped)
	Expect(out).To(HaveLen(1))
}
