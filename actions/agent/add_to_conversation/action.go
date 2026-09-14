// Package add_to_conversation stores a message in an agent conversation
// and returns the updated conversation history. Used in voice call subgraphs
// to maintain context between turns.
package add_to_conversation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Add to Conversation"
	Description  = "Store a message and return conversation history"
	Summary      = "Add a message to the conversation and get the history back"
	Website      = "https://www.flomation.co"
	Icon         = "comments"
	Date         = "29/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "conversation_id",
		Type:        core.ConnectionTypeString,
		Label:       "Conversation ID",
		Placeholder: "${conversation_id}",
		Required:    true,
	},
	{
		Name:        "agent_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent ID",
		Placeholder: "${flow.agent_id}",
		Required:    true,
	},
	{
		Name:     "content",
		Type:     core.ConnectionTypeText,
		Label:    "Message Content",
		Required: true,
	},
	{
		Name:  "role",
		Type:  core.ConnectionTypeString,
		Label: "Role",
		Options: []core.ConnectionOption{
			{Name: "User (inbound)", Value: "inbound"},
			{Name: "Assistant (outbound)", Value: "outbound"},
		},
		Required: true,
	},
	{
		Name:        "channel_type",
		Type:        core.ConnectionTypeString,
		Label:       "Channel Type",
		Placeholder: "${channel_type}",
	},
	{
		Name:        "sender",
		Type:        core.ConnectionTypeString,
		Label:       "Sender",
		Placeholder: "${from}",
	},
	{
		Name:        "history_limit",
		Type:        core.ConnectionTypeInteger,
		Label:       "History Limit",
		Placeholder: "30",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "conversation_history", Type: core.ConnectionTypeObject, Label: "Conversation History (AI format)"},
	{Name: "raw_history", Type: core.ConnectionTypeObject, Label: "Raw Conversation History"},
	{Name: "message_id", Type: core.ConnectionTypeString, Label: "Message ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	conversationID := optStr("conversation_id", inputs)
	agentID := optStr("agent_id", inputs)
	content := optStr("content", inputs)
	role := optStr("role", inputs)
	channelType := optStr("channel_type", inputs)
	sender := optStr("sender", inputs)
	historyLimit := optStr("history_limit", inputs)

	// The conversation id must be the PLATFORM's id for the conversation, not
	// the channel's. A Slack thread id or Telegram chat id passed straight
	// through from trigger data reaches the API as a malformed UUID and the
	// history request fails — which on live produced a stream of
	// "invalid input syntax for type uuid" against ids like "VijsyshjBd",
	// silently costing those flows their history.
	//
	// The execution context already carries the resolved id and the write path
	// in ai/record.go uses it, so fall back to that rather than failing: a flow
	// wired with ${conversation_id} from a trigger keeps working, and the id it
	// meant is the one it gets.
	ctx := flow.GetContext()
	if !isUUID(conversationID) {
		resolved := ""
		if ctx != nil {
			resolved = ctx.ConversationID
		}
		if resolved == "" {
			if conversationID == "" {
				return errResult("conversation_id is required — use ${flow.conversation_id}")
			}
			return errResult(fmt.Sprintf(
				"%q is not a Flomation conversation ID — it looks like an id from the channel (a Slack thread or Telegram chat). Use ${flow.conversation_id}, which is the platform's own id for this conversation", conversationID))
		}
		if conversationID != "" {
			log.WithFields(log.Fields{
				"supplied": conversationID,
				"resolved": resolved,
			}).Warn("add_to_conversation was given a non-UUID conversation id; using the execution context's id instead")
		}
		conversationID = resolved
	}

	if agentID == "" {
		return errResult("agent_id is required")
	}
	if content == "" {
		// Empty content — nothing to store, but still return history
		return fetchHistoryOnly(flow, conversationID, historyLimit)
	}
	if role == "" {
		role = "inbound"
	}

	if ctx == nil || ctx.APIURL == "" {
		return errResult("API URL not available")
	}

	// Store the message
	payload, _ := json.Marshal(map[string]interface{}{
		"direction":    role,
		"content":      content,
		"channel_type": channelType,
		"sender":       sender,
		"agent_id":     agentID,
	})

	storeURL := fmt.Sprintf("%s/api/v1/internal/conversation/%s/message?agent_id=%s",
		ctx.APIURL, conversationID, agentID)

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: ctx.InternalClient().Transport,
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, storeURL, bytes.NewReader(payload))
	if err != nil {
		return errResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("failed to store message: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	var storeResult struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&storeResult)

	// Fetch updated conversation history
	limit := "30"
	if historyLimit != "" {
		limit = historyLimit
	}

	historyURL := fmt.Sprintf("%s/api/v1/internal/conversation/%s/history?limit=%s",
		ctx.APIURL, conversationID, limit)

	histReq, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, historyURL, nil)
	if err != nil {
		return errResult(fmt.Sprintf("failed to create history request: %v", err))
	}

	histResp, err := client.Do(histReq)
	if err != nil {
		// Message was stored, just couldn't fetch history
		return map[string]interface{}{
			"tool_result":          fmt.Sprintf("Message stored (ID: %s) but failed to fetch history", storeResult.ID),
			"conversation_history": nil,
			"raw_history":          nil,
			"message_id":           storeResult.ID,
			"success":              true,
			"error":                "",
		}, nil
	}
	defer func() { _ = histResp.Body.Close() }()

	histBody, _ := io.ReadAll(io.LimitReader(histResp.Body, 1<<20))
	aiHistory, rawHistory := parseAndTransformHistory(histBody)

	return map[string]interface{}{
		"tool_result":          fmt.Sprintf("Message stored. Conversation has %d turns.", len(aiHistory)),
		"conversation_history": aiHistory,
		"raw_history":          rawHistory,
		"message_id":           storeResult.ID,
		"success":              true,
		"error":                "",
	}, nil
}

func fetchHistoryOnly(flow *core.Flow, conversationID, limit string) (map[string]interface{}, error) {
	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return errResult("API URL not available")
	}

	if limit == "" {
		limit = "30"
	}

	historyURL := fmt.Sprintf("%s/api/v1/internal/conversation/%s/history?limit=%s",
		ctx.APIURL, conversationID, limit)

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: ctx.InternalClient().Transport,
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, historyURL, nil)
	if err != nil {
		return errResult(fmt.Sprintf("failed to fetch history: %v", err))
	}

	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("failed to fetch history: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	aiHistory, rawHistory := parseAndTransformHistory(body)

	return map[string]interface{}{
		"tool_result":          fmt.Sprintf("Conversation history fetched (%d turns).", len(aiHistory)),
		"conversation_history": aiHistory,
		"raw_history":          rawHistory,
		"message_id":           "",
		"success":              true,
		"error":                "",
	}, nil
}

func optStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":          "Error: " + msg,
		"conversation_history": nil,
		"raw_history":          nil,
		"message_id":           "",
		"success":              false,
		"error":                msg,
	}, nil
}

// parseAndTransformHistory parses the API's AgentMessage array and returns
// both an AI-formatted history (role/content pairs) and the raw messages.
func parseAndTransformHistory(body []byte) ([]map[string]string, []interface{}) {
	var rawMessages []map[string]interface{}
	if err := json.Unmarshal(body, &rawMessages); err != nil {
		return nil, nil
	}

	// Keep raw as-is for the raw_history output
	var rawHistory []interface{}
	for _, m := range rawMessages {
		rawHistory = append(rawHistory, m)
	}

	// Transform to AI format: direction → role, skip tool_use/tool_result
	var aiHistory []map[string]string
	for _, m := range rawMessages {
		direction, _ := m["direction"].(string)
		content, _ := m["content"].(string)

		if content == "" {
			continue
		}

		// Skip tool exchange messages
		if direction == "tool_use" || direction == "tool_result" {
			continue
		}

		role := "user"
		if direction == "outbound" {
			role = "assistant"
		}

		aiHistory = append(aiHistory, map[string]string{
			"role":    role,
			"content": content,
		})
	}

	return aiHistory, rawHistory
}

// isUUID reports whether s is a canonical 8-4-4-4-12 hex UUID.
//
// Deliberately a shape check rather than a full parse: the only thing that
// matters here is telling a platform conversation id apart from a channel's
// own identifier before the API rejects it with a database error.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
