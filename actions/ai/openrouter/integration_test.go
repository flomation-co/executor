package openrouter

import (
	"os"
	"strconv"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// TestIntegrationLiveCompletion exercises the real OpenRouter endpoint
// end-to-end through Execute. It is skipped unless OPENROUTER_API_KEY is
// set, so it never runs in CI without credentials.
func TestIntegrationLiveCompletion(t *testing.T) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		t.Skip("OPENROUTER_API_KEY not set; skipping live OpenRouter integration test")
	}
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: key},
		{Name: "model", Type: core.ConnectionTypeString, Value: "anthropic/claude-haiku-4.5"},
		{Name: "system_prompt", Type: core.ConnectionTypeText, Value: "You are terse."},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Reply with exactly: OpenRouter works."},
		{Name: "max_tokens", Type: core.ConnectionTypeInteger, Value: float64(40)},
		{Name: "temperature", Type: core.ConnectionTypeString, Value: "0.1"},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(true))
	Expect(result["should_respond"]).To(Equal(true))
	Expect(result["model"]).To(ContainSubstring("claude"))
	Expect(result["response"]).To(ContainSubstring("OpenRouter works"))
	Expect(result["total_tokens"]).To(BeNumerically(">", int64(0)))
	// OpenRouter-specific outputs: the winning upstream provider and the
	// per-request cost in USD.
	Expect(result["provider"]).To(Not(BeEmpty()))
	cost, parseErr := strconv.ParseFloat(result["cost"].(string), 64)
	Expect(parseErr).To(BeNil())
	Expect(cost).To(BeNumerically(">", 0.0))
}

// TestIntegrationLiveToolCall exercises the first leg of the tool loop
// against the real endpoint, including the zero-parameter-tool case that
// historically made Anthropic-via-OpenRouter return "" arguments.
func TestIntegrationLiveToolCall(t *testing.T) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		t.Skip("OPENROUTER_API_KEY not set; skipping live OpenRouter integration test")
	}
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeString, Value: key},
		{Name: "model", Type: core.ConnectionTypeString, Value: "anthropic/claude-haiku-4.5"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "What time is it? You must use the get_time tool."},
		{Name: "max_tokens", Type: core.ConnectionTypeInteger, Value: float64(300)},
		{Name: "tool_definitions", Type: core.ConnectionTypeText, Value: `[{"type":"function","function":{"name":"get_time","description":"Get the current time","parameters":{"type":"object","properties":{}}}}]`},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["stop_reason"]).To(Equal("tool_calls"))

	requests, ok := result[core.ToolRequestsKey].([]core.ToolRequest)
	Expect(ok).To(BeTrue())
	Expect(len(requests)).To(BeNumerically(">=", 1))
	Expect(requests[0].Name).To(Equal("get_time"))
	// Zero-parameter tool: input must be an empty (non-nil) map whether the
	// provider returned "{}" or the historical "".
	Expect(requests[0].Input).To(Not(BeNil()))
	Expect(result[core.ToolConversationStateKey]).To(Not(BeNil()))
}
