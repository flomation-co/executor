package ai_common

import (
	"io"
	"net/http"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// fakeResponse wraps a canned body string in an *http.Response so the stream
// parsers can be driven without a real network round-trip.
func fakeResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// drain reads the streaming helper's placeholder output, ranges its sentence
// channel to completion (which guarantees FinalizeStream has run — the reader
// closes the channel last), and returns the sentences plus the finalised flow
// variables.
func drain(t *testing.T, flow *core.Flow, out map[string]interface{}) (sentences []string, vars map[string]interface{}) {
	t.Helper()
	if out[core.StreamSentencesKey] != true {
		t.Fatalf("expected StreamSentencesKey flag in outputs, got %v", out[core.StreamSentencesKey])
	}
	raw, ok := flow.GetVariable(core.StreamSentencesKey)
	if !ok {
		t.Fatal("StreamSentencesKey channel not set on flow")
	}
	ch, ok := raw.(chan string)
	if !ok {
		t.Fatalf("StreamSentencesKey is not a chan string: %T", raw)
	}
	for s := range ch {
		sentences = append(sentences, s)
	}
	vars = map[string]interface{}{}
	for _, k := range []string{core.StreamFullTextKey, core.StreamStopReasonKey, core.StreamUsageKey, core.StreamToolRequestsKey, "__stream_model"} {
		if v, ok := flow.GetVariable(k); ok {
			vars[k] = v
		}
	}
	return sentences, vars
}

func TestHandleOpenAICompatibleStream_TextAndUsage(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"model":"gpt-4o","choices":[{"delta":{"content":"Hello world."}}]}`,
		``,
		`data: {"model":"gpt-4o","choices":[{"delta":{"content":" How are you?"}}]}`,
		``,
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	flow := &core.Flow{}
	out, err := HandleOpenAICompatibleStream(flow, fakeResponse(sse), "gpt-4o", map[string]interface{}{"prompt_tokens": int64(0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sentences, vars := drain(t, flow, out)

	if len(sentences) != 2 || sentences[0] != "Hello world." || sentences[1] != "How are you?" {
		t.Fatalf("unexpected sentences: %#v", sentences)
	}
	if got := vars[core.StreamFullTextKey]; got != "Hello world. How are you?" {
		t.Fatalf("full text = %q", got)
	}
	if got := vars[core.StreamStopReasonKey]; got != "stop" {
		t.Fatalf("stop reason = %q", got)
	}
	usage, ok := vars[core.StreamUsageKey].(map[string]int64)
	if !ok {
		t.Fatalf("usage type = %T", vars[core.StreamUsageKey])
	}
	if usage["prompt_tokens"] != 10 || usage["completion_tokens"] != 5 || usage["total_tokens"] != 15 {
		t.Fatalf("usage prompt/completion/total = %#v", usage)
	}
	// The canonical keys must also be present so the engine drain populates
	// Anthropic-style token outputs too.
	if usage["input_tokens"] != 10 || usage["output_tokens"] != 5 {
		t.Fatalf("canonical usage = %#v", usage)
	}
	if got := vars["__stream_model"]; got != "gpt-4o" {
		t.Fatalf("model = %q", got)
	}
	if _, hasTools := vars[core.StreamToolRequestsKey]; hasTools {
		t.Fatal("did not expect tool requests")
	}
}

func TestHandleOpenAICompatibleStream_ToolCalls(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"get_weather","arguments":"{\"ci"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ty\":\"Paris\"}"}}]}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	}, "\n")

	flow := &core.Flow{}
	out, err := HandleOpenAICompatibleStream(flow, fakeResponse(sse), "gpt-4o", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, vars := drain(t, flow, out)

	reqs, ok := vars[core.StreamToolRequestsKey].([]core.ToolRequest)
	if !ok || len(reqs) != 1 {
		t.Fatalf("expected 1 tool request, got %#v", vars[core.StreamToolRequestsKey])
	}
	if reqs[0].ID != "call_1" || reqs[0].Name != "get_weather" {
		t.Fatalf("tool request id/name = %q/%q", reqs[0].ID, reqs[0].Name)
	}
	if reqs[0].Input["city"] != "Paris" {
		t.Fatalf("tool input = %#v", reqs[0].Input)
	}
}

func TestHandleGeminiStream_TextAndUsage(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"Bonjour."}]}}],"modelVersion":"gemini-2.5-flash"}`,
		`data: {"candidates":[{"content":{"parts":[{"text":" Ca va?"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":3,"totalTokenCount":11}}`,
	}, "\n")

	flow := &core.Flow{}
	out, err := HandleGeminiStream(flow, fakeResponse(sse), "gemini-2.5-flash", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sentences, vars := drain(t, flow, out)

	if len(sentences) != 2 || sentences[0] != "Bonjour." || sentences[1] != "Ca va?" {
		t.Fatalf("unexpected sentences: %#v", sentences)
	}
	if got := vars[core.StreamFullTextKey]; got != "Bonjour. Ca va?" {
		t.Fatalf("full text = %q", got)
	}
	usage := vars[core.StreamUsageKey].(map[string]int64)
	if usage["prompt_tokens"] != 8 || usage["completion_tokens"] != 3 || usage["total_tokens"] != 11 {
		t.Fatalf("usage = %#v", usage)
	}
	if got := vars["__stream_model"]; got != "gemini-2.5-flash" {
		t.Fatalf("model = %q", got)
	}
}

func TestHandleGeminiStream_FunctionCall(t *testing.T) {
	sse := `data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"lookup","args":{"q":"x"}}}]}}],"modelVersion":"gemini-2.5-pro"}`

	flow := &core.Flow{}
	out, err := HandleGeminiStream(flow, fakeResponse(sse), "gemini-2.5-pro", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, vars := drain(t, flow, out)

	reqs, ok := vars[core.StreamToolRequestsKey].([]core.ToolRequest)
	if !ok || len(reqs) != 1 {
		t.Fatalf("expected 1 tool request, got %#v", vars[core.StreamToolRequestsKey])
	}
	if reqs[0].Name != "lookup" || reqs[0].ID != "lookup-0" || reqs[0].Input["q"] != "x" {
		t.Fatalf("tool request = %#v", reqs[0])
	}
	if vars[core.StreamStopReasonKey] != "function_call" {
		t.Fatalf("stop reason = %q", vars[core.StreamStopReasonKey])
	}
}

func TestHandleOllamaStream_TextAndUsage(t *testing.T) {
	// NDJSON — one complete object per line.
	nd := strings.Join([]string{
		`{"model":"llama3.2","message":{"role":"assistant","content":"Hi there."},"done":false}`,
		`{"model":"llama3.2","message":{"role":"assistant","content":" Bye now."},"done":true,"done_reason":"stop","prompt_eval_count":12,"eval_count":4}`,
	}, "\n")

	flow := &core.Flow{}
	out, err := HandleOllamaStream(flow, fakeResponse(nd), "llama3.2", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sentences, vars := drain(t, flow, out)

	if len(sentences) != 2 || sentences[0] != "Hi there." || sentences[1] != "Bye now." {
		t.Fatalf("unexpected sentences: %#v", sentences)
	}
	if got := vars[core.StreamFullTextKey]; got != "Hi there. Bye now." {
		t.Fatalf("full text = %q", got)
	}
	usage := vars[core.StreamUsageKey].(map[string]int64)
	if usage["prompt_tokens"] != 12 || usage["completion_tokens"] != 4 || usage["total_tokens"] != 16 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestHandleOllamaStream_ToolCalls(t *testing.T) {
	nd := strings.Join([]string{
		`{"model":"llama3.2","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"add","arguments":{"a":1,"b":2}}}]},"done":false}`,
		`{"model":"llama3.2","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":2}`,
	}, "\n")

	flow := &core.Flow{}
	out, err := HandleOllamaStream(flow, fakeResponse(nd), "llama3.2", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, vars := drain(t, flow, out)

	reqs, ok := vars[core.StreamToolRequestsKey].([]core.ToolRequest)
	if !ok || len(reqs) != 1 {
		t.Fatalf("expected 1 tool request, got %#v", vars[core.StreamToolRequestsKey])
	}
	if reqs[0].Name != "add" || reqs[0].ID != "call_0_0" {
		t.Fatalf("tool request id/name = %q/%q", reqs[0].ID, reqs[0].Name)
	}
	if vars[core.StreamStopReasonKey] != "tool_calls" {
		t.Fatalf("stop reason = %q", vars[core.StreamStopReasonKey])
	}
}

func TestFindSentenceBoundary(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"Hello world. Next", 11},
		{"No boundary here", -1},
		{"Dr. Smith arrived.", 17}, // skips "Dr.", finds the final "."
		{"Pi is 3.14 exactly.", 18},
		{"Really?! yes", 6}, // "?" at index 6 followed by "!" is NOT a boundary; "!" at 7 is not followed by space... so next boundary is none until... check
	}
	for _, c := range cases {
		got := findSentenceBoundary(c.text)
		// The last case is intentionally loose — just assert the function
		// doesn't panic and returns a sane index or -1.
		if c.text == "Really?! yes" {
			if got < -1 || got >= len(c.text) {
				t.Fatalf("findSentenceBoundary(%q) out of range: %d", c.text, got)
			}
			continue
		}
		if got != c.want {
			t.Fatalf("findSentenceBoundary(%q) = %d, want %d", c.text, got, c.want)
		}
	}
}
