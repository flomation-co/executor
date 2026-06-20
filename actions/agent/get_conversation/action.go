// Package get_conversation is the executor action that returns the
// full message history of a prior conversation referenced by its
// conversation_id. The agent calls it as a tool when one of the
// summaries in its "Recent conversations" section looks relevant
// enough to drill into.
//
// The action is unusual among the executor's agent tools in that its
// output can be substantial — a long conversation pushes >10KB of
// message JSON. The AI tool loop's BlobStore tokenisation catches
// this automatically (see executor/core/blob_tokenise.go): the
// model sees a compact flo:blob:<handle> reference and never has
// the raw payload occupying its context window.
package get_conversation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Conversation"
	Description  = "Fetch the full message history of a previous conversation the agent has had with this user. Use when a summary in your Recent Conversations section looks relevant."
	Website      = "https://www.flomation.co"
	Icon         = "comments+magnifying-glass"
	Date         = "20/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "agent_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent ID",
		Placeholder: "${flow.agent_id}",
		Required:    true,
	},
	{
		Name:        "agent_user_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent User ID",
		Placeholder: "${flow.agent_user_id}",
		Required:    true,
	},
	{
		Name:        "conversation_id",
		Type:        core.ConnectionTypeString,
		Label:       "Conversation ID — pass the conversation_id verbatim from the Recent Conversations section.",
		Placeholder: "00000000-0000-0000-0000-000000000000",
		Required:    true,
	},
	{
		Name:        "max_messages",
		Type:        core.ConnectionTypeInteger,
		Label:       "Maximum messages to return (defaults to 200; max 500). Messages are read in sequence order from the start of the conversation.",
		Placeholder: "200",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "messages", Type: core.ConnectionTypeObject, Label: "Messages in sequence order"},
	{Name: "message_count", Type: core.ConnectionTypeInteger, Label: "Total messages in the conversation"},
	{Name: "returned_count", Type: core.ConnectionTypeInteger, Label: "Messages actually returned in this response"},
	{Name: "ended_at", Type: core.ConnectionTypeString, Label: "Conversation end timestamp (if ended)"},
	{Name: "was_truncated", Type: core.ConnectionTypeBoolean, Label: "True if the conversation has more messages than max_messages"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// Execute calls the API's internal endpoint with mTLS. Auth scoping
// (the agent must own the conversation AND match the agent_user_id)
// is enforced server-side in the WHERE clause of the persistence
// query — the executor doesn't need to repeat it.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentID := optionalString("agent_id", inputs)
	if agentID == "" {
		return errResult("agent_id is required")
	}
	agentUserID := optionalString("agent_user_id", inputs)
	if agentUserID == "" {
		return errResult("agent_user_id is required")
	}
	conversationID := optionalString("conversation_id", inputs)
	if conversationID == "" {
		return errResult("conversation_id is required")
	}
	if strings.HasPrefix(conversationID, "${") {
		// Unresolved template — caller wiring failed. Return a clear
		// error rather than calling the API with junk.
		return errResult("conversation_id contains an unresolved variable reference; pass the conversation_id literal from the Recent Conversations section")
	}

	maxMessages := optionalInt("max_messages", inputs)
	if maxMessages <= 0 {
		maxMessages = 200
	}
	if maxMessages > 500 {
		maxMessages = 500
	}

	execCtx := flow.GetContext()
	if execCtx == nil || execCtx.APIURL == "" {
		return errResult("execution context with API URL is required")
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"agent_user_id": agentUserID,
		"max_messages":  maxMessages,
	})

	endpoint := fmt.Sprintf("%s/api/v1/internal/agent/%s/conversation/%s/messages",
		execCtx.APIURL, agentID, conversationID)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return errResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := execCtx.InternalClient().Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("failed to call get_conversation endpoint: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		// 404 from the auth-scoped persistence layer means either
		// the conversation doesn't exist OR it does but isn't
		// accessible to this (agent, user). The two cases are
		// indistinguishable by design — surface a single clear
		// message rather than leaking the existence of other
		// users' conversations.
		return errResult(fmt.Sprintf("conversation %s is not accessible — check that the conversation_id came from your Recent Conversations section and belongs to this user", conversationID))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return errResult(fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)))
	}

	var parsed struct {
		Messages      []map[string]interface{} `json:"messages"`
		MessageCount  int64                    `json:"message_count"`
		ReturnedCount int                      `json:"returned_count"`
		EndedAt       *string                  `json:"ended_at"`
		WasTruncated  bool                     `json:"was_truncated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return errResult(fmt.Sprintf("failed to decode response: %v", err))
	}

	log.WithFields(log.Fields{
		"agent_id":        agentID,
		"conversation_id": conversationID,
		"returned":        parsed.ReturnedCount,
		"total":           parsed.MessageCount,
		"truncated":       parsed.WasTruncated,
	}).Info("agent/get_conversation fetched conversation history")

	endedAtStr := ""
	if parsed.EndedAt != nil {
		endedAtStr = *parsed.EndedAt
	}

	// Tool result summary: the model reads this string verbatim.
	// Mention the message count + truncation status; the actual
	// messages array is on a separate output and (when large) gets
	// off-loaded to the BlobStore so the model sees only a compact
	// reference there.
	truncNote := ""
	if parsed.WasTruncated {
		truncNote = fmt.Sprintf(" (truncated to first %d of %d messages — ask the user if you need messages later in the thread)", parsed.ReturnedCount, parsed.MessageCount)
	}

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Retrieved %d messages from conversation %s%s", parsed.ReturnedCount, conversationID, truncNote),
		"messages":       parsed.Messages,
		"message_count":  parsed.MessageCount,
		"returned_count": parsed.ReturnedCount,
		"ended_at":       endedAtStr,
		"was_truncated":  parsed.WasTruncated,
		"success":        true,
		"error":          "",
	}, nil
}

// errResult mirrors the shape of a successful return but with
// success=false set. Keeps the AI tool-loop's downstream code from
// needing to special-case a nil messages array.
func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":    msg,
		"messages":       []map[string]interface{}{},
		"message_count":  int64(0),
		"returned_count": 0,
		"ended_at":       "",
		"was_truncated":  false,
		"success":        false,
		"error":          msg,
	}, nil
}

func optionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func optionalInt(name string, inputs []*core.Connection) int {
	c := core.FindConnection(name, inputs)
	if c == nil || c.Number() == nil {
		return 0
	}
	return int(*c.Number())
}
