package ai_common

import (
	"encoding/json"
	"strings"
)

// Message is the common chat-message shape used by both OpenAI and Anthropic
// (role + content). Used for passing conversation history into AI actions.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ParseConversationHistory normalises the value of a conversation_history
// input into a slice of Messages. The input value may be nil, a JSON string,
// or an already-parsed slice (either []Message, []map[string]interface{},
// []map[string]string, or []interface{}). Unknown shapes return nil.
func ParseConversationHistory(raw interface{}) []Message {
	if raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case []Message:
		return v
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return nil
		}
		var msgs []Message
		if err := json.Unmarshal([]byte(v), &msgs); err == nil {
			return msgs
		}
		return nil
	case []map[string]string:
		out := make([]Message, 0, len(v))
		for _, m := range v {
			out = append(out, Message{Role: m["role"], Content: m["content"]})
		}
		return out
	case []map[string]interface{}:
		out := make([]Message, 0, len(v))
		for _, m := range v {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			out = append(out, Message{Role: role, Content: content})
		}
		return out
	case []interface{}:
		out := make([]Message, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				role, _ := m["role"].(string)
				content, _ := m["content"].(string)
				out = append(out, Message{Role: role, Content: content})
			} else if m, ok := item.(Message); ok {
				out = append(out, m)
			}
		}
		return out
	}

	return nil
}

// approxTokens estimates the token count of a string using a conservative
// characters-per-token heuristic. This avoids a runtime dependency on a real
// tokeniser (tiktoken etc) while giving us a safe upper bound for truncation.
// The ratio of ~4 chars/token is the widely-used approximation for English.
func approxTokens(s string) int {
	return (len(s) + 3) / 4
}

// ApproxMessageTokens estimates the token cost of a single message. A small
// overhead is added to account for role tags and message boundaries in the
// underlying API wire format.
func ApproxMessageTokens(m Message) int {
	const messageOverhead = 8
	return approxTokens(m.Role) + approxTokens(m.Content) + messageOverhead
}

// TruncateHistoryForBudget drops the oldest messages from history until the
// total estimated token cost of (history + systemPrompt + userPrompt + buffer
// for the reply) fits within modelContext. The caller supplies:
//
//   - history:       messages ordered oldest → newest
//   - systemPrompt:  system instructions (may be empty)
//   - userPrompt:    the new user turn about to be appended
//   - maxTokens:     max_tokens reserved for the model's reply
//   - modelContext:  the target model's total context window in tokens
//
// Returns the (possibly shortened) history. If modelContext is zero or
// negative, history is returned unchanged — the caller has opted out of
// truncation.
func TruncateHistoryForBudget(history []Message, systemPrompt, userPrompt string, maxTokens, modelContext int) []Message {
	if modelContext <= 0 || len(history) == 0 {
		return history
	}

	fixedCost := approxTokens(systemPrompt) + approxTokens(userPrompt) + maxTokens + 64 // safety margin
	budget := modelContext - fixedCost
	if budget <= 0 {
		// Prompt alone already exceeds budget — nothing we can do with history.
		return nil
	}

	total := 0
	for _, m := range history {
		total += ApproxMessageTokens(m)
	}
	if total <= budget {
		return history
	}

	// Drop from the front (oldest) until we fit.
	drop := 0
	for drop < len(history) && total > budget {
		total -= ApproxMessageTokens(history[drop])
		drop++
	}
	return history[drop:]
}

// ModelContextWindow returns a conservative context-window size (in tokens)
// for a given model identifier. Unknown models fall back to a safe default.
// These numbers are intentionally conservative — the goal is to prevent
// 400/413 errors from the provider, not to maximise utilisation.
func ModelContextWindow(model string) int {
	m := strings.ToLower(model)
	switch {
	// Anthropic Claude
	case strings.Contains(m, "claude-opus-4-6") && strings.Contains(m, "1m"):
		return 1000000
	case strings.Contains(m, "claude-opus-4-6"),
		strings.Contains(m, "claude-sonnet-4-6"),
		strings.Contains(m, "claude-haiku-4-5"):
		return 200000
	case strings.Contains(m, "claude-"):
		return 200000
	// OpenAI
	case strings.Contains(m, "gpt-4.1"):
		return 1000000
	case strings.Contains(m, "gpt-4o"), strings.Contains(m, "o3"), strings.Contains(m, "o4"):
		return 128000
	case strings.Contains(m, "gpt-4"):
		return 128000
	case strings.Contains(m, "gpt-3.5"):
		return 16000
	}
	return 32000 // safe default
}
