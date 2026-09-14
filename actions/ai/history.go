package ai_common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

// Automatic conversation history retrieval.
//
// # The asymmetry this fixes
//
// Storing an agent's conversation is transparent: Launch records the inbound
// message, RecordAssistantReply records the assistant turn, and
// RecordToolExchange records tool calls. Nobody wires any of that.
//
// Reading it back was not. Before this, an AI action only saw history if the
// flow author wired an Add to Conversation node upstream and passed its output
// into the conversation_history input. Leave it blank and the model is handed
// nothing — no error, no warning, just an agent that has forgotten everything
// since its last message.
//
// On live that was 12 of 14 agents. The platform had faithfully recorded 20,722
// messages across 1,150 conversations and was handing back none of them. The
// symptom is not obvious from the outside: the agent does not say it has lost
// the thread, it confidently redoes work it already did, and any value it
// regenerates from memory (a URL, a reference, an identifier) drifts a little
// each time.
//
// # The rule
//
// An explicit input always wins, so every flow that wires history today behaves
// exactly as it did. The fetch only happens when the input is ABSENT — which is
// the case that used to mean "silently no history". Setting the input to an
// empty array is therefore a deliberate opt-out for a flow that manages its own
// history, which is what a long-running flow emitting several turns per
// execution needs.
//
// The conversation is identified by ExecutionContext.ConversationID, which the
// platform resolves and which the WRITE path already uses. Deriving it from a
// trigger field instead is what produces the malformed-id failures described in
// add_to_conversation.

const (
	historyTimeout = 8 * time.Second

	// defaultHistoryTurns is what a turn of conversation is worth bringing
	// back. TruncateHistoryForBudget trims further to fit the model's context
	// window, so this is a ceiling on the request rather than on what the model
	// sees; it exists to bound the response body rather than the prompt.
	defaultHistoryTurns = 30
)

// ResolveConversationHistory returns the history for this turn.
//
// supplied is the raw value of the action's conversation_history input. When it
// carries anything at all — including an explicitly empty list — it is
// authoritative and no request is made. Only an absent input triggers a fetch.
func ResolveConversationHistory(flowCtx context.Context, ctx *core.ExecutionContext, supplied interface{}) []Message {
	if historySupplied(supplied) {
		return ParseConversationHistory(supplied)
	}
	return FetchConversationHistory(flowCtx, ctx, defaultHistoryTurns)
}

// historySupplied reports whether the flow author gave the action a value.
//
// The distinction that matters is "absent" versus "present but empty". A nil
// input, an empty string, or an unresolved ${...} reference are all absent — the
// last because a reference that did not resolve is the exact shape of the bug
// this fixes, and treating it as a deliberate empty history would preserve it.
func historySupplied(supplied interface{}) bool {
	switch v := supplied.(type) {
	case nil:
		return false
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" || strings.HasPrefix(trimmed, "${") {
			return false
		}
		return true
	default:
		// A non-string value (an array or object arriving from an upstream
		// node) is a real answer, even if it is an empty list.
		return true
	}
}

// FetchConversationHistory reads this conversation's turns from the API.
//
// No-ops — returning nil rather than an error — under the same conditions as
// RecordAssistantReply, so the read and write paths agree on what "in an agent
// context" means:
//
//   - ctx is nil (running outside an execution, as in unit tests)
//   - AgentID is empty (an ordinary flow that happens to use an AI action)
//   - ConversationID is empty (an agent turn with no conversation, e.g. a
//     schedule-triggered orchestrator rather than a channel message)
//   - APIURL is empty (no API reachable)
//
// A failure is logged and treated as "no history". Failing the AI action would
// be worse: the user gets nothing at all instead of an answer that lacks
// context, and the previous behaviour was no history in every case anyway.
func FetchConversationHistory(flowCtx context.Context, ctx *core.ExecutionContext, limit int) []Message {
	if ctx == nil || ctx.AgentID == "" || ctx.ConversationID == "" || ctx.APIURL == "" {
		return nil
	}
	if limit <= 0 {
		limit = defaultHistoryTurns
	}

	reqCtx, cancel := context.WithTimeout(flowCtx, historyTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/internal/conversation/%s/history?limit=%d",
		ctx.APIURL, ctx.ConversationID, limit)

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		log.WithFields(log.Fields{"agent_id": ctx.AgentID, "error": err}).
			Warn("unable to build the conversation history request")
		return nil
	}
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		log.WithFields(log.Fields{"agent_id": ctx.AgentID, "error": err}).
			Warn("unable to fetch conversation history — this turn runs without it")
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"agent_id":        ctx.AgentID,
			"conversation_id": ctx.ConversationID,
			"status":          resp.StatusCode,
		}).Warn("conversation history request was refused — this turn runs without it")
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHistoryBytes))
	if err != nil {
		log.WithFields(log.Fields{"agent_id": ctx.AgentID, "error": err}).
			Warn("unable to read the conversation history response")
		return nil
	}

	messages := transformHistory(body)
	log.WithFields(log.Fields{
		"agent_id":        ctx.AgentID,
		"conversation_id": ctx.ConversationID,
		"turns":           len(messages),
	}).Debug("conversation history fetched automatically")
	return messages
}

// maxHistoryBytes bounds the response. Thirty turns of chat is small; a cap
// stops a pathological conversation from being read entirely into memory before
// TruncateHistoryForBudget has a chance to trim it.
const maxHistoryBytes = 4 << 20

// transformHistory converts stored messages into the role/content pairs the
// providers expect.
//
// Mirrors the mapping in agent/add_to_conversation deliberately: a message's
// direction is the platform's own vocabulary (inbound/outbound), and tool
// exchanges are recorded in the same table but are not conversation turns —
// including them would present the model with tool chatter as if the user had
// said it.
func transformHistory(body []byte) []Message {
	var raw []map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	messages := make([]Message, 0, len(raw))
	for _, m := range raw {
		content, _ := m["content"].(string)
		if content == "" {
			continue
		}
		direction, _ := m["direction"].(string)
		if direction == "tool_use" || direction == "tool_result" {
			continue
		}

		role := "user"
		if direction == "outbound" {
			role = "assistant"
		}
		messages = append(messages, Message{Role: role, Content: content})
	}
	return messages
}

// ConversationHistoryFor resolves the history for an action from its
// conversation_history connection, which may be absent.
//
// This is the form the provider actions call: it keeps the auto-fetch decision
// in one place rather than repeated across the eight chat providers, each of
// which would otherwise need the same nil-check-then-fallback dance.
func ConversationHistoryFor(flow *core.Flow, conn *core.Connection) []Message {
	var supplied interface{}
	if conn != nil {
		supplied = conn.Value
	}
	if flow == nil {
		return ParseConversationHistory(supplied)
	}
	return ResolveConversationHistory(flow.GoContext(), flow.GetContext(), supplied)
}
