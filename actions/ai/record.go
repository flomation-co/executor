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
		"channel_type": "assistant",
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

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/message", ctx.APIURL, ctx.AgentID)
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

	client := &http.Client{Timeout: recordTimeout}
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
}
