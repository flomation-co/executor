// Package slack_rich_message is an AI agent tool for sending rich Slack
// messages with Block Kit layouts, attachments, and mrkdwn formatting.
//
// The AI constructs blocks dynamically based on the conversation context.
// Simple text replies should use the standard messaging/slack action via
// the Response handle — this tool is for when the agent needs structured
// layouts (sections with fields, images, buttons, dividers, context blocks).
package slack_rich_message

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
	Name         = "Slack Rich Message"
	Description  = "Send a rich Slack message with Block Kit layouts. Use this when you need structured formatting: " +
		"sections with fields, images, buttons, dividers, or context blocks. For simple text replies, " +
		"use the normal response instead. Blocks use Slack mrkdwn (*bold*, _italic_, ~strike~, `code`). " +
		"IMPORTANT: bot_token, channel_id, and thread_ts are pre-configured — do NOT provide them and do NOT ask the user for them. " +
		"You only need to provide: text (fallback) and blocks (the Block Kit JSON array)."
	Website = "https://www.flomation.co"
	Icon    = "slack+file-lines"
	Date    = "15/04/2026"
	Type    = core.ActionTypeAction

	slackAPIBase = "https://slack.com/api"
)

var Inputs = [...]core.Connection{
	{
		Name:        "bot_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Bot Token",
		Placeholder: "xoxb-...",
	},
	{
		Name:        "channel_id",
		// Not a secret: a channel ID is in every Slack URL, and the four other
		// Slack actions already type it as a string. Typed as a secret it was
		// masked in the execution view, hiding the one value you need to see
		// when Slack answers channel_not_found.
		Type:        core.ConnectionTypeString,
		Label:       "Channel ID",
		Placeholder: "${channel_id}",
	},
	{
		Name:     "text",
		Type:     core.ConnectionTypeText,
		Label:    "Fallback text shown in notifications and accessibility (plain summary of the message)",
		Required: true,
	},
	{
		Name: "blocks",
		Type: core.ConnectionTypeText,
		Label: "Block Kit blocks as a JSON array (a bare [...] array, or the Block Kit Builder's {\"blocks\":[...]} object — both accepted). " +
			"Common block types: section (with mrkdwn text or fields), divider, image, context, header, actions (buttons). " +
			"Example: [{\"type\":\"header\",\"text\":{\"type\":\"plain_text\",\"text\":\"Report\"}},{\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"*Status:* All clear\"}}]",
		Required: true,
	},
	{
		Name:  "thread_ts",
		Type:  core.ConnectionTypeString,
		Label: "Thread timestamp to reply in (optional)",
	},
	{
		Name:  "attachments",
		Type:  core.ConnectionTypeText,
		Label: "Legacy attachments JSON array — colour bars, fields, footers (optional)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary for the AI"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Message timestamp"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// Auto-inherit bot_token and channel_id from the flow context if not
	// provided by the AI. This means agents never need to ask the user for
	// these values — they come from the orchestrator flow's trigger data.
	botToken := requireString("bot_token", inputs)
	if botToken == "" {
		// Check flow variables (set by trigger data or orchestrator).
		for _, key := range []string{"bot_token", "slack_bot_token"} {
			if v, ok := flow.GetVariable(key); ok {
				if s, ok := v.(string); ok && s != "" {
					botToken = s
					break
				}
			}
		}
	}
	if botToken == "" {
		// Check trigger data — the entire trigger payload is available
		// as flow variables via ${trigger.key} syntax.
		if v, ok := flow.GetVariable("trigger.bot_token"); ok {
			if s, ok := v.(string); ok && s != "" {
				botToken = s
			}
		}
	}
	if botToken == "" {
		return map[string]interface{}{
			"tool_result": "Bot token not available — ensure the Slack bot token is configured on this tool node in the flow editor.",
			"success":     false,
			"error":       "bot_token not configured",
		}, nil
	}

	channelID := requireString("channel_id", inputs)
	if channelID == "" {
		// Fall back to flow context channel_id (set by trigger data).
		if ctx := flow.GetContext(); ctx != nil && ctx.ChannelID != "" {
			channelID = ctx.ChannelID
		}
	}
	if channelID == "" {
		return map[string]interface{}{
			"tool_result": "Channel ID not available — this tool auto-inherits the channel from the conversation context.",
			"success":     false,
			"error":       "channel_id not available",
		}, nil
	}
	text := requireString("text", inputs)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	blocksRaw := optionalString("blocks", inputs)
	if blocksRaw == "" {
		return nil, fmt.Errorf("blocks is required")
	}

	// Parse blocks JSON — leniently (see parseBlockKitArray).
	blocks, err := parseBlockKitArray(blocksRaw)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Invalid blocks JSON: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}

	payload := map[string]interface{}{
		"channel": channelID,
		"text":    text,
		"mrkdwn":  true,
		"blocks":  blocks,
	}

	threadTS := optionalString("thread_ts", inputs)
	// Validate thread_ts format: Slack timestamps look like "1234567890.123456".
	// The AI sometimes passes channel IDs, "null", or other invalid values.
	if threadTS != "" && !isValidSlackTS(threadTS) {
		log.WithField("thread_ts", threadTS).Warn("ignoring invalid thread_ts from AI")
		threadTS = ""
	}
	if threadTS == "" {
		// Fall back to flow context thread_id.
		if ctx := flow.GetContext(); ctx != nil && ctx.ThreadID != "" {
			threadTS = ctx.ThreadID
		}
	}
	if threadTS != "" {
		payload["thread_ts"] = threadTS
	}

	// Parse optional attachments (also accepts the {"attachments":[...]} wrapper).
	if attachRaw := optionalString("attachments", inputs); attachRaw != "" {
		if attachments, err := parseJSONArrayOrWrapped(attachRaw, "attachments"); err != nil {
			log.WithError(err).Warn("failed to parse attachments JSON — skipping")
		} else {
			payload["attachments"] = attachments
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, slackAPIBase+"/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to send: %s", err),
			"success":     false,
			"error":       err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var result struct {
		OK    bool   `json:"ok"`
		TS    string `json:"ts"`
		Error string `json:"error"`
	}
	_ = json.Unmarshal(respBody, &result)

	if !result.OK {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Slack API error: %s", result.Error),
			"timestamp":   "",
			"success":     false,
			"error":       result.Error,
		}, nil
	}

	blockCount := len(blocks)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Rich message sent with %d blocks", blockCount),
		"timestamp":   result.TS,
		"success":     true,
		"error":       "",
	}, nil
}

// parseBlockKitArray parses the blocks/attachments input leniently. Slack's own
// Block Kit Builder exports the wrapper object {"blocks":[...]} (and likewise
// {"attachments":[...]}), which people paste verbatim — so as well as the bare
// JSON array we also unwrap that object under the given key. Markdown code
// fences the AI sometimes adds are stripped first.
func parseBlockKitArray(raw string) ([]interface{}, error) {
	return parseJSONArrayOrWrapped(raw, "blocks")
}

func parseJSONArrayOrWrapped(raw, wrapperKey string) ([]interface{}, error) {
	cleaned := stripCodeFence(strings.TrimSpace(raw))

	// Preferred shape: a bare JSON array.
	var arr []interface{}
	arrErr := json.Unmarshal([]byte(cleaned), &arr)
	if arrErr == nil {
		return arr, nil
	}

	// Builder wrapper: {"<wrapperKey>":[...]} — unwrap it.
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(cleaned), &obj) == nil {
		if inner, ok := obj[wrapperKey]; ok {
			if err := json.Unmarshal(inner, &arr); err == nil {
				return arr, nil
			}
		}
	}

	// Surface the original array-parse error — it's the most actionable.
	return nil, arrErr
}

func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if idx := strings.Index(s[3:], "\n"); idx != -1 {
		s = s[3+idx+1:]
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

// isValidSlackTS checks whether a string looks like a Slack message timestamp
// (e.g. "1234567890.123456"). The AI sometimes passes channel IDs, "null", or
// empty strings which cause invalid_thread_ts errors from the Slack API.
func isValidSlackTS(ts string) bool {
	if len(ts) < 5 {
		return false
	}
	dotIdx := strings.IndexByte(ts, '.')
	if dotIdx < 1 {
		return false
	}
	// Both parts should be numeric
	for _, c := range ts {
		if c != '.' && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func requireString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

func optionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}
