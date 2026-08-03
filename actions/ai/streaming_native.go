package ai_common

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
)

// === Native-format streaming (Gemini, Ollama) ===
//
// Gemini and Ollama don't speak the OpenAI Chat Completions SSE format, so
// they each need their own parse loop. Both still drive the shared
// SentenceStreamer and finalise the same engine contract via FinalizeStream,
// so the executor consumes them identically to the OpenAI-family and
// Anthropic streams.

// HandleGeminiStream parses a Gemini streamGenerateContent SSE stream
// (?alt=sse), splitting candidate text into sentences and accumulating any
// functionCall parts as tool requests, then finalises the engine streaming
// contract. Gemini gives function calls no stable id, so — matching the
// non-streaming path — each is keyed "name-index". The goroutine owns
// resp.Body.
func HandleGeminiStream(flow *core.Flow, resp *http.Response, model string, extraOutputs map[string]interface{}) (map[string]interface{}, error) {
	streamer := NewSentenceStreamer()

	// Publish the channel before spawning the reader — see the note in
	// HandleOpenAICompatibleStream.
	flow.SetVariable(core.StreamSentencesKey, streamer.Channel())

	go func() {
		defer streamer.Close()
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		var toolRequests []core.ToolRequest
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

			var event struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text         string `json:"text"`
							FunctionCall *struct {
								Name string                 `json:"name"`
								Args map[string]interface{} `json:"args"`
							} `json:"functionCall,omitempty"`
						} `json:"parts"`
					} `json:"content"`
					FinishReason string `json:"finishReason"`
				} `json:"candidates"`
				ModelVersion  string `json:"modelVersion"`
				UsageMetadata *struct {
					PromptTokenCount     int64 `json:"promptTokenCount"`
					CandidatesTokenCount int64 `json:"candidatesTokenCount"`
					TotalTokenCount      int64 `json:"totalTokenCount"`
				} `json:"usageMetadata,omitempty"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if event.ModelVersion != "" {
				respModel = event.ModelVersion
			}
			if event.UsageMetadata != nil {
				promptTokens = event.UsageMetadata.PromptTokenCount
				completionTokens = event.UsageMetadata.CandidatesTokenCount
				totalTokens = event.UsageMetadata.TotalTokenCount
			}
			for _, cand := range event.Candidates {
				for _, part := range cand.Content.Parts {
					if part.Text != "" {
						streamer.PushText(part.Text)
					}
					if part.FunctionCall != nil && part.FunctionCall.Name != "" {
						streamer.FlushPending()
						toolRequests = append(toolRequests, core.ToolRequest{
							ID:    fmt.Sprintf("%s-%d", part.FunctionCall.Name, len(toolRequests)),
							Name:  part.FunctionCall.Name,
							Input: part.FunctionCall.Args,
						})
					}
				}
				if cand.FinishReason != "" {
					stopReason = cand.FinishReason
				}
			}
		}
		streamer.FlushPending()

		if respModel == "" {
			respModel = model
		}
		if len(toolRequests) > 0 {
			stopReason = "function_call"
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

// HandleOllamaStream parses an Ollama /api/chat streaming response. Unlike
// the SSE providers, Ollama streams newline-delimited JSON (one complete
// object per line), each carrying an incremental message.content delta; the
// terminal object (done=true) carries the token counts. Tool calls arrive
// whole (arguments as an object, no id) so — matching the non-streaming
// path — each is keyed "call_0_index". The goroutine owns resp.Body.
func HandleOllamaStream(flow *core.Flow, resp *http.Response, model string, extraOutputs map[string]interface{}) (map[string]interface{}, error) {
	streamer := NewSentenceStreamer()

	// Publish the channel before spawning the reader — see the note in
	// HandleOpenAICompatibleStream.
	flow.SetVariable(core.StreamSentencesKey, streamer.Channel())

	go func() {
		defer streamer.Close()
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		var toolRequests []core.ToolRequest
		var stopReason, respModel string
		var promptTokens, completionTokens int64

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var event struct {
				Model   string `json:"model"`
				Message struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Function struct {
							Name      string                 `json:"name"`
							Arguments map[string]interface{} `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls,omitempty"`
				} `json:"message"`
				Done            bool   `json:"done"`
				DoneReason      string `json:"done_reason"`
				PromptEvalCount int64  `json:"prompt_eval_count"`
				EvalCount       int64  `json:"eval_count"`
			}
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				continue
			}
			if event.Model != "" {
				respModel = event.Model
			}
			if event.Message.Content != "" {
				streamer.PushText(event.Message.Content)
			}
			for _, tc := range event.Message.ToolCalls {
				if tc.Function.Name == "" {
					continue
				}
				streamer.FlushPending()
				input := tc.Function.Arguments
				if input == nil {
					input = map[string]interface{}{}
				}
				toolRequests = append(toolRequests, core.ToolRequest{
					ID:    fmt.Sprintf("call_0_%d", len(toolRequests)),
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			if event.Done {
				if event.DoneReason != "" {
					stopReason = event.DoneReason
				}
				promptTokens = event.PromptEvalCount
				completionTokens = event.EvalCount
			}
		}
		streamer.FlushPending()

		if respModel == "" {
			respModel = model
		}
		if len(toolRequests) > 0 {
			stopReason = "tool_calls"
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
				"total_tokens":      promptTokens + completionTokens,
			},
			ToolRequests: toolRequests,
		})
	}()

	return StreamingInitialOutputs(model, extraOutputs), nil
}
