package ollama

import (
	"encoding/json"
	"os"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// Live tests are skipped unless OLLAMA_ENDPOINT is set (e.g.
// http://localhost:11434), so CI never needs a local model server. The
// model defaults to qwen3:0.6b — small enough for CPU, and it supports
// thinking, tools, and JSON mode — and can be overridden with OLLAMA_MODEL.
func liveConfig(t *testing.T) (endpoint, model string) {
	endpoint = os.Getenv("OLLAMA_ENDPOINT")
	if endpoint == "" {
		t.Skip("OLLAMA_ENDPOINT not set; skipping live Ollama integration test")
	}
	model = os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "qwen3:0.6b"
	}
	return endpoint, model
}

func TestIntegrationLiveCompletion(t *testing.T) {
	endpoint, model := liveConfig(t)
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
		{Name: "model", Type: core.ConnectionTypeString, Value: model},
		{Name: "system_prompt", Type: core.ConnectionTypeText, Value: "You are terse."},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Reply with exactly: Ollama works."},
		{Name: "max_tokens", Type: core.ConnectionTypeInteger, Value: float64(2000)},
		{Name: "temperature", Type: core.ConnectionTypeString, Value: "0.1"},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(true))
	Expect(result["should_respond"]).To(Equal(true))
	Expect(result["model"]).To(Equal(model))
	Expect(result["response"]).To(ContainSubstring("Ollama works"))
	Expect(result["total_tokens"]).To(BeNumerically(">", int64(0)))
}

// TestIntegrationLiveThinking verifies the top-level think flag round-trip:
// thinking output populated when on, absent when off.
func TestIntegrationLiveThinking(t *testing.T) {
	endpoint, model := liveConfig(t)
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
		{Name: "model", Type: core.ConnectionTypeString, Value: model},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "What is 2+2? Answer with just the number."},
		{Name: "think", Type: core.ConnectionTypeString, Value: "on"},
		{Name: "max_tokens", Type: core.ConnectionTypeInteger, Value: float64(2000)},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(true))
	Expect(result["thinking"]).To(Not(BeEmpty()))

	// think=off must suppress the thinking trace entirely.
	for i, c := range inputs {
		if c.Name == "think" {
			inputs[i] = &core.Connection{Name: "think", Type: core.ConnectionTypeString, Value: "off"}
		}
	}
	// The answer's correctness is the model's problem, not the action's —
	// tiny test models flub arithmetic without thinking. Assert only that
	// the trace is suppressed and a response still arrives.
	result, err = Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["thinking"]).To(BeEmpty())
	Expect(result["response"]).To(Not(BeEmpty()))
}

// TestIntegrationLiveJSONFormat verifies the top-level format field: the
// response must parse as JSON. (n8n nests format inside options, where the
// server ignores it — this asserts our top-level placement actually works.)
func TestIntegrationLiveJSONFormat(t *testing.T) {
	endpoint, model := liveConfig(t)
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
		{Name: "model", Type: core.ConnectionTypeString, Value: model},
		{Name: "think", Type: core.ConnectionTypeString, Value: "off"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: `List two colours as json: {"colours": [...]}`},
		{Name: "response_format", Type: core.ConnectionTypeString, Value: "json"},
		{Name: "max_tokens", Type: core.ConnectionTypeInteger, Value: float64(500)},
	}

	result, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	var parsed map[string]interface{}
	Expect(json.Unmarshal([]byte(result["response"].(string)), &parsed)).To(BeNil())
}

// TestIntegrationLiveToolLoop exercises BOTH legs of the tool loop against
// a real server: the first leg's native id-less tool_calls with object
// arguments, and the re-entry leg's tool_name result mapping.
func TestIntegrationLiveToolLoop(t *testing.T) {
	endpoint, model := liveConfig(t)
	RegisterTestingT(t)

	toolDefs := `[{"type":"function","function":{"name":"get_weather","description":"Get the current weather for a city","parameters":{"type":"object","properties":{"city":{"type":"string","description":"City name"}},"required":["city"]}}}]`

	inputs := []*core.Connection{
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
		{Name: "model", Type: core.ConnectionTypeString, Value: model},
		{Name: "think", Type: core.ConnectionTypeString, Value: "off"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "What is the weather in London? You must use the get_weather tool."},
		{Name: "max_tokens", Type: core.ConnectionTypeInteger, Value: float64(1000)},
		{Name: "tool_definitions", Type: core.ConnectionTypeText, Value: toolDefs},
	}

	flow := &core.Flow{}
	result, err := Execute(flow, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["stop_reason"]).To(Equal("tool_calls"))

	requests, ok := result[core.ToolRequestsKey].([]core.ToolRequest)
	Expect(ok).To(BeTrue())
	Expect(len(requests)).To(BeNumerically(">=", 1))
	Expect(requests[0].Name).To(Equal("get_weather"))
	Expect(requests[0].ID).To(Not(BeEmpty()))
	Expect(requests[0].Input).To(HaveKey("city"))

	// Second leg: feed the tool result back the way the engine would.
	state := result[core.ToolConversationStateKey]
	Expect(state).To(Not(BeNil()))
	flow.SetVariable(core.ToolConversationStateKey, state)
	flow.SetVariable(core.ToolResultsKey, []core.ToolResult{
		{ToolUseID: requests[0].ID, Content: "sunny, 25C"},
	})

	result, err = Execute(flow, nil, inputs)
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(true))
	Expect(result["response"]).To(ContainSubstring("unny")) // sunny/Sunny
}
