// Package search_conversation is the executor agent tool that full-text
// searches the agent's ENTIRE conversation history with the current user —
// across all channels and past conversations. The agent calls it when a summary
// or its compacted context isn't enough to resolve a reference ("what did I say
// about X", a terse "try again"): it searches storage rather than relying on the
// context window, which only holds a truncated/summarised slice.
package search_conversation

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
	Name         = "Search Conversation History"
	Description  = "Full-text search your entire message history with this user (all channels) to recall something outside your current context. Provide a search query."
	Website      = "https://www.flomation.co"
	Icon         = "comments+magnifying-glass"
	Date         = "13/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "agent_id", Type: core.ConnectionTypeString, Label: "Agent ID", Placeholder: "${flow.agent_id}", Required: true},
	{Name: "agent_user_id", Type: core.ConnectionTypeString, Label: "Agent User ID", Placeholder: "${flow.agent_user_id}", Required: true},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search query — words or a phrase to find in past messages.", Placeholder: "the demo wallpaper image", Required: true},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Max results", Placeholder: "20"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Matching messages"},
	{Name: "match_count", Type: core.ConnectionTypeInteger, Label: "Match Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentID := str("agent_id", inputs)
	agentUserID := str("agent_user_id", inputs)
	query := strings.TrimSpace(str("query", inputs))
	if agentID == "" || agentUserID == "" {
		return errResult("agent_id and agent_user_id are required (wire them to ${flow.agent_id} / ${flow.agent_user_id})")
	}
	if query == "" {
		return errResult("query is required")
	}
	limit := optionalInt("limit", inputs)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	execCtx := flow.GetContext()
	if execCtx == nil || execCtx.APIURL == "" {
		return errResult("execution context with API URL is required")
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"agent_user_id": agentUserID,
		"query":         query,
		"limit":         limit,
	})
	endpoint := fmt.Sprintf("%s/api/v1/internal/agent/%s/history/search", execCtx.APIURL, agentID)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return errResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := execCtx.InternalClient().Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("failed to call search endpoint: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return errResult(fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)))
	}

	var parsed struct {
		Results []struct {
			ConversationID string `json:"conversation_id"`
			ChannelType    string `json:"channel_type"`
			Direction      string `json:"direction"`
			Sender         string `json:"sender"`
			Content        string `json:"content"`
			CreatedAt      string `json:"created_at"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return errResult(fmt.Sprintf("failed to decode response: %v", err))
	}

	log.WithFields(log.Fields{
		"agent_id": agentID,
		"matches":  len(parsed.Results),
	}).Info("agent/search_conversation searched history")

	if len(parsed.Results) == 0 {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("No messages found matching %q in your history with this user.", query),
			"results":     []interface{}{},
			"match_count": int64(0),
			"success":     true,
			"error":       "",
		}, nil
	}

	// Human/LLM-readable summary the model reads verbatim. Each line: who
	// said it, when, on which channel, and the message (truncated).
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d message(s) matching %q:\n", len(parsed.Results), query)
	for _, r := range parsed.Results {
		who := "user"
		if r.Direction == "outbound" {
			who = "you"
		} else if r.Direction == "system" {
			who = "system"
		}
		content := r.Content
		if len(content) > 500 {
			content = content[:500] + "…"
		}
		fmt.Fprintf(&b, "- [%s · %s · %s] %s\n", r.CreatedAt, r.ChannelType, who, content)
	}

	return map[string]interface{}{
		"tool_result": b.String(),
		"results":     parsed.Results,
		"match_count": int64(len(parsed.Results)),
		"success":     true,
		"error":       "",
	}, nil
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"results":     []interface{}{},
		"match_count": int64(0),
		"success":     false,
		"error":       msg,
	}, nil
}

func str(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return strings.TrimSpace(*c.String())
}

func optionalInt(name string, inputs []*core.Connection) int64 {
	c := core.FindConnection(name, inputs)
	if c == nil || c.Number() == nil {
		return 0
	}
	return *c.Number()
}
