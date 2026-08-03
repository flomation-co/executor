package ai_common

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
)

// === Shared streaming machinery ===
//
// Every AI provider that supports low-latency streaming produces the same
// engine contract: a channel of complete sentences (drained by the executor
// to fire the Response handle per sentence for TTS), plus a set of flow
// variables carrying the accumulated full text, stop reason, token usage,
// model, and any tool requests. Only the wire format of the upstream SSE
// stream differs per provider. This file owns the provider-agnostic half —
// sentence splitting, the channel lifecycle, and the flow-variable
// finalisation — so each provider's action only has to parse its own SSE
// events and drive a SentenceStreamer.

// SentenceStreamer accumulates streamed text, emitting complete sentences to
// a buffered channel for low-latency TTS while retaining the full text for
// the final response. Not safe for concurrent use — a single goroutine owns
// it for the lifetime of one stream.
type SentenceStreamer struct {
	ch             chan string
	sentenceBuffer strings.Builder
	fullText       strings.Builder
}

// NewSentenceStreamer returns a streamer with a small buffered channel so a
// slow TTS consumer doesn't stall the SSE read loop for a sentence or two.
func NewSentenceStreamer() *SentenceStreamer {
	return &SentenceStreamer{ch: make(chan string, 10)}
}

// Channel is the sentence channel to hand to the executor via
// core.StreamSentencesKey.
func (s *SentenceStreamer) Channel() chan string { return s.ch }

// PushText appends a text delta, emitting any newly-complete sentences.
func (s *SentenceStreamer) PushText(delta string) {
	if delta == "" {
		return
	}
	s.sentenceBuffer.WriteString(delta)
	s.fullText.WriteString(delta)
	emitSentences(&s.sentenceBuffer, s.ch)
}

// FlushPending emits any buffered partial sentence. Call it before switching
// to tool-call accumulation and once more at stream end so the trailing
// fragment (which may have no sentence-ending punctuation) still reaches the
// consumer.
func (s *SentenceStreamer) FlushPending() {
	remaining := strings.TrimSpace(s.sentenceBuffer.String())
	if remaining != "" {
		s.ch <- remaining
		s.sentenceBuffer.Reset()
	}
}

// FullText returns everything streamed so far.
func (s *SentenceStreamer) FullText() string { return s.fullText.String() }

// Close closes the sentence channel. Call via defer in the streaming
// goroutine so the executor's drain loop terminates.
func (s *SentenceStreamer) Close() { close(s.ch) }

// StreamResult carries the terminal metadata gathered while streaming, used
// to populate the engine's streaming flow-variable contract.
type StreamResult struct {
	FullText     string
	StopReason   string
	Model        string
	InputTokens  int64
	OutputTokens int64
	// ExtraUsage lets OpenAI-family providers carry their native token keys
	// (prompt_tokens/completion_tokens/total_tokens) through the drain in
	// addition to the canonical input_tokens/output_tokens.
	ExtraUsage   map[string]int64
	ToolRequests []core.ToolRequest
}

// FinalizeStream writes the engine's streaming flow-variable contract so the
// executor's drainStreamingChannel picks up the results after the sentence
// channel is drained. Call it from the streaming goroutine after Close().
func FinalizeStream(flow *core.Flow, r StreamResult) {
	flow.SetVariable(core.StreamFullTextKey, r.FullText)
	flow.SetVariable(core.StreamStopReasonKey, r.StopReason)

	usage := map[string]int64{
		"input_tokens":  r.InputTokens,
		"output_tokens": r.OutputTokens,
	}
	for k, v := range r.ExtraUsage {
		usage[k] = v
	}
	flow.SetVariable(core.StreamUsageKey, usage)

	if len(r.ToolRequests) > 0 {
		flow.SetVariable(core.StreamToolRequestsKey, r.ToolRequests)
	}
	if r.Model != "" {
		flow.SetVariable("__stream_model", r.Model)
	}
}

// StreamingInitialOutputs returns the placeholder output map an AI action
// returns synchronously the moment it kicks off streaming. The real values
// (response, tokens, model, stop_reason, tool requests) are filled in by the
// executor after the sentence channel drains. Provider-specific fields
// (e.g. zeroed prompt_tokens for OpenAI-family outputs) come in via extra.
func StreamingInitialOutputs(model string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		core.StreamSentencesKey: true,
		"response":              "",
		"model":                 model,
		"should_respond":        true,
		"tool_calls_count":      int64(0),
		"success":               true,
		"error":                 "",
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// HandleOpenAICompatibleStream parses an OpenAI Chat Completions SSE stream
// (shared verbatim by OpenAI, Groq, OpenRouter, Azure OpenAI and Open WebUI),
// splitting assistant text into sentences and accumulating any tool calls,
// then finalises the engine streaming contract. The returned map is the
// action's synchronous placeholder output; extraOutputs are merged in so the
// provider's declared token fields exist on the node before the drain fills
// them. The goroutine owns resp.Body and closes it.
//
// To receive token usage on the final chunk the caller must set
// `stream_options.include_usage = true` in the request payload; without it
// the usage fields stay zero (harmless — the response text is unaffected).
func HandleOpenAICompatibleStream(flow *core.Flow, resp *http.Response, model string, extraOutputs map[string]interface{}) (map[string]interface{}, error) {
	streamer := NewSentenceStreamer()

	// Publish the channel BEFORE spawning the reader so the main goroutine's
	// write and the reader's later FinalizeStream writes never touch the
	// flow's variable map concurrently (the `go` statement synchronises).
	flow.SetVariable(core.StreamSentencesKey, streamer.Channel())

	go func() {
		defer streamer.Close()
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		// Tool calls arrive incrementally, keyed by index: the id and name
		// land in the first delta for an index, arguments accumulate across
		// subsequent deltas. order preserves first-seen sequence.
		type toolAcc struct {
			id   string
			name string
			args strings.Builder
		}
		toolCalls := map[int]*toolAcc{}
		var order []int

		var stopReason, respModel string
		var promptTokens, completionTokens, totalTokens int64

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "" {
				continue
			}
			if data == "[DONE]" {
				break
			}

			var event struct {
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content   string `json:"content"`
						ToolCalls []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls,omitempty"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens     int64 `json:"prompt_tokens"`
					CompletionTokens int64 `json:"completion_tokens"`
					TotalTokens      int64 `json:"total_tokens"`
				} `json:"usage,omitempty"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if event.Model != "" {
				respModel = event.Model
			}
			if event.Usage != nil {
				promptTokens = event.Usage.PromptTokens
				completionTokens = event.Usage.CompletionTokens
				totalTokens = event.Usage.TotalTokens
			}
			for _, choice := range event.Choices {
				if choice.Delta.Content != "" {
					streamer.PushText(choice.Delta.Content)
				}
				for _, tc := range choice.Delta.ToolCalls {
					acc := toolCalls[tc.Index]
					if acc == nil {
						// First fragment of a new tool call — flush any
						// pending text before we switch modes.
						streamer.FlushPending()
						acc = &toolAcc{}
						toolCalls[tc.Index] = acc
						order = append(order, tc.Index)
					}
					if tc.ID != "" {
						acc.id = tc.ID
					}
					if tc.Function.Name != "" {
						acc.name = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						acc.args.WriteString(tc.Function.Arguments)
					}
				}
				if choice.FinishReason != "" {
					stopReason = choice.FinishReason
				}
			}
		}
		streamer.FlushPending()

		var toolRequests []core.ToolRequest
		for _, idx := range order {
			acc := toolCalls[idx]
			if acc == nil || acc.name == "" {
				continue
			}
			var input map[string]interface{}
			if acc.args.Len() > 0 {
				_ = json.Unmarshal([]byte(acc.args.String()), &input)
			}
			if input == nil {
				input = make(map[string]interface{})
			}
			toolRequests = append(toolRequests, core.ToolRequest{
				ID:    acc.id,
				Name:  acc.name,
				Input: input,
			})
		}

		if respModel == "" {
			respModel = model
		}
		FinalizeStream(flow, StreamResult{
			FullText:     streamer.FullText(),
			StopReason:   stopReason,
			Model:        respModel,
			InputTokens:  promptTokens,
			OutputTokens: completionTokens,
			ExtraUsage: map[string]int64{
				"prompt_tokens":     promptTokens,
				"completion_tokens": completionTokens,
				"total_tokens":      totalTokens,
			},
			ToolRequests: toolRequests,
		})
	}()

	return StreamingInitialOutputs(model, extraOutputs), nil
}

// emitSentences drains complete sentences from buf into ch, leaving any
// trailing partial sentence in buf. Shared by every provider's stream path.
func emitSentences(buf *strings.Builder, ch chan<- string) {
	text := buf.String()
	for {
		idx := findSentenceBoundary(text)
		if idx < 0 {
			break
		}
		sentence := strings.TrimSpace(text[:idx+1])
		if sentence != "" {
			ch <- sentence
		}
		text = text[idx+1:]
	}
	buf.Reset()
	buf.WriteString(text)
}

// findSentenceBoundary returns the index of the first sentence-ending
// character (. ! ?) followed by whitespace or end of text, or -1 if none.
// Common abbreviations and decimals are skipped so a sentence isn't cut mid
// "Dr." or "3.14".
func findSentenceBoundary(text string) int {
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch != '.' && ch != '!' && ch != '?' {
			continue
		}
		// Must be followed by whitespace or end of string.
		if i+1 < len(text) && text[i+1] != ' ' && text[i+1] != '\n' {
			continue
		}
		// Skip common abbreviations (Mr. Mrs. Dr. etc.).
		if ch == '.' && i >= 2 {
			before := strings.ToLower(text[max(0, i-3) : i+1])
			if strings.HasSuffix(before, "mr.") ||
				strings.HasSuffix(before, "mrs.") ||
				strings.HasSuffix(before, "ms.") ||
				strings.HasSuffix(before, "dr.") ||
				strings.HasSuffix(before, "st.") ||
				strings.HasSuffix(before, "no.") {
				continue
			}
		}
		// Skip decimal numbers (3.14).
		if ch == '.' && i > 0 && i+1 < len(text) {
			if text[i-1] >= '0' && text[i-1] <= '9' && text[i+1] >= '0' && text[i+1] <= '9' {
				continue
			}
		}
		return i
	}
	return -1
}
