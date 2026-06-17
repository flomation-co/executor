package ai_common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

// recordTimeout bounds the outbound-record API call so a slow or hung API
// cannot stall the AI action. The AI action's own API call has just returned
// successfully at this point, so we know the model was happy — recording is
// a bookkeeping side-effect and must never block the action's success path.
const recordTimeout = 5 * time.Second

// RecordAssistantReply posts the AI action's response to the API as an
// outbound agent_message when this execution is running inside an agent
// orchestrator flow (i.e. ExecutionContext.AgentID is set).
//
// This is what keeps the conversation_history balanced across turns. Without
// it, the agent_message table accumulates only inbound user turns (because
// the messaging/slack and messaging/telegram delivery actions do not record)
// and the next turn's history contains N consecutive user messages with no
// assistant replies between them. OpenAI and Anthropic both interpret that
// as "here are N user statements, please address them all", producing huge
// responses that never move on to the latest request — the conversation
// loop bug.
//
// Recording happens here in the AI action (not in the downstream delivery
// action) because the AI's response IS the assistant turn by platform
// definition, regardless of which delivery action eventually sends it out
// to the channel. Any delivery action can be used without breaking history.
//
// Failures are logged but never surfaced to the caller. A failed record is
// a non-fatal bookkeeping miss; failing the AI action would be worse because
// the user-visible response has already been generated.
//
// No-ops (returns nil) in the following cases:
//   - ctx is nil (action running outside an execution context — shouldn't
//     happen in production but is safe in unit tests)
//   - ctx.AgentID is empty (not in an agent context — normal flows that use
//     AI actions outside of agent orchestration)
//   - ctx.APIURL is empty (no API reachable — test contexts)
//   - content is empty (nothing to record)
func RecordAssistantReply(flowCtx context.Context, ctx *core.ExecutionContext, content string) {
	if ctx == nil || ctx.AgentID == "" || ctx.APIURL == "" || content == "" {
		return
	}

	payload, err := json.Marshal(map[string]interface{}{
		"direction":    "outbound",
		"channel_type": ctx.ChannelType,
		"sender":       "agent",
		"content":      content,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"agent_id": ctx.AgentID,
			"error":    err,
		}).Warn("failed to marshal assistant reply for recording")
		return
	}

	// Use a bounded timeout rather than relying on the flow context alone,
	// because the flow context may be cancellation-free for an entire flow
	// execution and we don't want a slow API to stall the AI action.
	reqCtx, cancel := context.WithTimeout(flowCtx, recordTimeout)
	defer cancel()

	// Use the conversation-scoped endpoint when a conversation_id is
	// available. This stores the outbound message with the correct
	// conversation_id, session_id, and sequence so the conversation
	// history includes both user and assistant turns. Falls back to
	// the legacy agent-wide endpoint for non-conversation contexts.
	var url string
	if ctx.ConversationID != "" {
		url = fmt.Sprintf("%s/api/v1/internal/conversation/%s/message?agent_id=%s",
			ctx.APIURL, ctx.ConversationID, ctx.AgentID)
	} else {
		url = fmt.Sprintf("%s/api/v1/internal/agent/%s/message", ctx.APIURL, ctx.AgentID)
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		log.WithFields(log.Fields{
			"agent_id": ctx.AgentID,
			"error":    err,
		}).Warn("failed to build assistant reply record request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	client := &http.Client{
		Timeout:   recordTimeout,
		Transport: ctx.InternalClient().Transport,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"agent_id": ctx.AgentID,
			"error":    err,
		}).Warn("failed to record assistant reply")
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.WithFields(log.Fields{
			"agent_id": ctx.AgentID,
			"status":   resp.StatusCode,
		}).Warn("assistant reply record returned non-2xx")
		return
	}

	// Phase 2d-γ: after recording the assistant reply, fire the
	// extraction pipeline on the assistant's response text. This
	// is what lets the extraction flow detect commitments the
	// agent makes ("I'll come back to you in an hour") and
	// assistant-derived memory updates. The extraction call is
	// fire-and-forget — any failure only costs one turn of
	// extraction, not the user's reply.
	//
	// Skip extraction when:
	// - This is a system flow execution (e.g. the extraction flow
	//   itself) — prevents infinite cascading loops
	// - This is a commitment-triggered execution — prevents the
	//   follow-up reply from creating duplicate commitments
	if !ctx.SystemFlow && ctx.TriggerSource != "commitment" && ctx.TriggerSource != "pending_action" {
		dispatchAssistantExtraction(flowCtx, ctx, content)
	}
}

// ToolExchange represents a single tool invocation within an AI turn:
// the request the model made and the result it received.
type ToolExchange struct {
	ToolUseID string                 `json:"tool_use_id"`
	Name      string                 `json:"name"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Result    string                 `json:"result"`
	IsError   bool                   `json:"is_error,omitempty"`
}

// RecordToolExchange stores the intermediate tool_use and tool_result
// messages from a completed AI turn. This fills the gap that caused the
// "context loss" bug: without these records, the next turn's conversation
// history only contained the user's text and the assistant's text — the
// model had no memory of which tools it called or what they returned.
//
// Each exchange produces two agent_message rows:
//   - direction=tool_use  — what the model asked to call (name + input)
//   - direction=tool_result — what came back (result text)
//
// The content field stores a human-readable summary; the metadata JSONB
// field stores the structured data so the history normaliser in Launch
// can reconstruct proper Anthropic/OpenAI tool message formats.
//
// Same fire-and-forget contract as RecordAssistantReply.
func RecordToolExchange(flowCtx context.Context, ctx *core.ExecutionContext, exchanges []ToolExchange) {
	if ctx == nil || ctx.AgentID == "" || ctx.APIURL == "" || ctx.ConversationID == "" || len(exchanges) == 0 {
		return
	}

	client := &http.Client{
		Timeout:   recordTimeout,
		Transport: ctx.InternalClient().Transport,
	}
	apiEndpoint := fmt.Sprintf("%s/api/v1/internal/conversation/%s/message?agent_id=%s",
		ctx.APIURL, ctx.ConversationID, ctx.AgentID)

	for _, ex := range exchanges {
		// 1. Record the tool_use message (what the model called)
		inputJSON, _ := json.Marshal(ex.Input)
		useContent := fmt.Sprintf("[Tool Call] %s(%s)", ex.Name, string(inputJSON))
		usePayload, err := json.Marshal(map[string]interface{}{
			"direction":    "tool_use",
			"channel_type": ctx.ChannelType,
			"sender":       "agent",
			"content":      useContent,
			"metadata": map[string]interface{}{
				"tool_use_id": ex.ToolUseID,
				"tool_name":   ex.Name,
				"tool_input":  ex.Input,
			},
		})
		if err != nil {
			continue
		}

		reqCtx, cancel := context.WithTimeout(flowCtx, recordTimeout)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, apiEndpoint, bytes.NewReader(usePayload)) // #nosec G107 — internal service URL
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if ctx.Token != "" {
			req.Header.Set("Authorization", "Bearer "+ctx.Token)
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 256))
			resp.Body.Close()
		}
		cancel()

		// 2. Record the tool_result message (what came back)
		resultContent := ex.Result
		if len(resultContent) > 2000 {
			resultContent = resultContent[:2000] + "… [truncated]"
		}
		resultPayload, err := json.Marshal(map[string]interface{}{
			"direction":    "tool_result",
			"channel_type": ctx.ChannelType,
			"sender":       "system",
			"content":      fmt.Sprintf("[Tool Result] %s → %s", ex.Name, resultContent),
			"metadata": map[string]interface{}{
				"tool_use_id": ex.ToolUseID,
				"tool_name":   ex.Name,
				"is_error":    ex.IsError,
				"raw_result":  ex.Result,
			},
		})
		if err != nil {
			continue
		}

		reqCtx2, cancel2 := context.WithTimeout(flowCtx, recordTimeout)
		req2, err := http.NewRequestWithContext(reqCtx2, http.MethodPost, apiEndpoint, bytes.NewReader(resultPayload)) // #nosec G107 — internal service URL
		if err != nil {
			cancel2()
			continue
		}
		req2.Header.Set("Content-Type", "application/json")
		if ctx.Token != "" {
			req2.Header.Set("Authorization", "Bearer "+ctx.Token)
		}
		resp2, err := client.Do(req2)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp2.Body, 256))
			resp2.Body.Close()
		}
		cancel2()
	}

	log.WithFields(log.Fields{
		"agent_id":        ctx.AgentID,
		"conversation_id": ctx.ConversationID,
		"exchanges":       len(exchanges),
	}).Debug("recorded tool exchanges in conversation history")
}

// dispatchAssistantExtraction calls POST /internal/agent/:id/extract
// with role=assistant and the assistant's reply content. This is the
// sibling of Launch's dispatchExtraction for inbound turns; together
// they ensure extraction runs on both halves of the conversation, as
// required by plans/agent_memory.md §"The extraction pipeline":
// "Running extraction on assistant replies as well as user turns is
// what makes commitment detection work."
//
// Same fire-and-forget contract as RecordAssistantReply: failures are
// logged and swallowed, the reply path is never blocked.
func dispatchAssistantExtraction(flowCtx context.Context, ctx *core.ExecutionContext, content string) {
	if ctx == nil || ctx.AgentID == "" || ctx.APIURL == "" || content == "" {
		return
	}

	body := map[string]interface{}{
		"role":    "assistant",
		"content": content,
	}
	if ctx.AgentUserID != "" {
		body["agent_user_id"] = ctx.AgentUserID
	}
	if ctx.ConversationID != "" {
		body["conversation_id"] = ctx.ConversationID
	}

	payload, err := json.Marshal(body)
	if err != nil {
		log.WithFields(log.Fields{
			"agent_id": ctx.AgentID,
			"error":    err,
		}).Warn("failed to marshal extraction payload for assistant turn")
		return
	}

	reqCtx, cancel := context.WithTimeout(flowCtx, recordTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/extract", ctx.APIURL, ctx.AgentID)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		log.WithFields(log.Fields{
			"agent_id": ctx.AgentID,
			"error":    err,
		}).Warn("failed to build assistant extraction request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	client := &http.Client{
		Timeout:   recordTimeout,
		Transport: ctx.InternalClient().Transport,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"agent_id": ctx.AgentID,
			"error":    err,
		}).Warn("failed to dispatch assistant extraction")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 256))

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		log.WithFields(log.Fields{
			"agent_id": ctx.AgentID,
			"status":   resp.StatusCode,
		}).Warn("unexpected response from assistant extraction dispatch")
	}
}
