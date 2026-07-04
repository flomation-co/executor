package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Send Telegram Message"
	Description  = "Send a message via the Telegram Bot API"
	Website      = "https://www.flomation.co"
	Icon         = "paper-plane"
	Date         = "03/04/2026"
	Type         = core.ActionTypeAction

	telegramAPIBase = "https://api.telegram.org"
	maxMessageLen   = 4096
)

var Inputs = [...]core.Connection{
	{
		Name:        "bot_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Bot Token",
		Placeholder: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		Required:    true,
	},
	{
		Name:        "channel_id",
		Type:        core.ConnectionTypeString,
		Label:       "Channel ID",
		Placeholder: "${flow.channel_id}",
		Required:    true,
	},
	{
		Name:        "message",
		Type:        core.ConnectionTypeText,
		Label:       "Message",
		Placeholder: "Hello from Flomation!",
		Required:    true,
	},
	{
		Name:  "parse_mode",
		Type:  core.ConnectionTypeString,
		Label: "Parse Mode",
		Options: []core.ConnectionOption{
			{Name: "None", Value: ""},
			{Name: "HTML", Value: "HTML"},
			{Name: "MarkdownV2", Value: "MarkdownV2"},
		},
	},
	{
		Name:        "reply_markup",
		Type:        core.ConnectionTypeText,
		Label:       "Reply Markup (JSON)",
		Placeholder: `{"inline_keyboard":[[{"text":"Approve","callback_data":"yes"}]]}`,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "message_id", Type: core.ConnectionTypeInteger, Label: "Message ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	botTokenConn := core.FindConnection("bot_token", inputs)
	if botTokenConn == nil || botTokenConn.String() == nil || *botTokenConn.String() == "" {
		return nil, fmt.Errorf("bot_token is required")
	}
	botToken := *botTokenConn.String()

	// Accept both canonical "channel_id" and legacy "chat_id" for
	// backwards compatibility with existing flows.
	chatIDConn := core.FindConnection("channel_id", inputs)
	if chatIDConn == nil || chatIDConn.String() == nil || *chatIDConn.String() == "" {
		chatIDConn = core.FindConnection("chat_id", inputs)
	}
	if chatIDConn == nil || chatIDConn.String() == nil || *chatIDConn.String() == "" {
		return nil, fmt.Errorf("channel_id is required")
	}
	chatID := *chatIDConn.String()

	// Guard against unresolved template variables ("${channel_id}",
	// "${chat_id}", "#{channel_id}", etc.) leaking through to Telegram's
	// API and being persisted as a literal string into agent_conversation.
	// These appear when a flow declares a placeholder using the wrong
	// namespace (e.g. ${channel_id} instead of ${flow.channel_id}) — the
	// substitution loop in flow.go silently leaves the literal in place
	// rather than erroring, so we must catch it here.
	if strings.HasPrefix(chatID, "${") || strings.HasPrefix(chatID, "#{") {
		return nil, fmt.Errorf("channel_id contains an unresolved template variable: %q — flow author likely used the wrong namespace (try ${flow.channel_id})", chatID)
	}

	messageConn := core.FindConnection("message", inputs)
	if messageConn == nil || messageConn.String() == nil || *messageConn.String() == "" {
		// Empty message — the AI likely communicated via tools and has no
		// final text to send. Skip gracefully instead of failing.
		return map[string]interface{}{
			"tool_result": "no message to send (empty response)",
			"message_id":  0,
			"success":     true,
			"error":       "",
		}, nil
	}
	message := *messageConn.String()

	parseModeConn := core.FindConnection("parse_mode", inputs)
	parseMode := ""
	if parseModeConn != nil && parseModeConn.String() != nil {
		parseMode = *parseModeConn.String()
	}

	// Optional inline keyboard / reply markup (e.g. from the Human-in-the-Loop
	// node). Attached only to the final chunk so the buttons sit beneath the
	// full text when a long message is split.
	var replyMarkup interface{}
	if rm := core.FindConnection("reply_markup", inputs); rm != nil && rm.String() != nil && strings.TrimSpace(*rm.String()) != "" {
		if err := json.Unmarshal([]byte(*rm.String()), &replyMarkup); err != nil {
			return nil, fmt.Errorf("reply_markup is not valid JSON: %w", err)
		}
	}

	// Telegram caps a single message at 4096 UTF-16 code units. Split long
	// messages into multiple sends on line/word boundaries rather than
	// silently truncating the overflow.
	chunks := splitTelegramMessage(message)
	var firstID int64
	for i, chunk := range chunks {
		payload := map[string]interface{}{"chat_id": chatID, "text": chunk}
		if parseMode != "" {
			payload["parse_mode"] = parseMode
		}
		if replyMarkup != nil && i == len(chunks)-1 {
			payload["reply_markup"] = replyMarkup
		}

		id, ok, desc, err := sendTelegramMessage(flow.GoContext(), botToken, payload)
		if err != nil {
			return map[string]interface{}{"message_id": nil, "success": false, "error": err.Error()}, nil
		}
		if !ok {
			return map[string]interface{}{"message_id": nil, "success": false, "error": desc}, nil
		}
		if i == 0 {
			firstID = id
		}
	}

	result := "message sent"
	if len(chunks) > 1 {
		result = fmt.Sprintf("message sent in %d parts", len(chunks))
	}
	return map[string]interface{}{
		"tool_result": result,
		"message_id":  firstID,
		"success":     true,
		"error":       "",
	}, nil
}

// sendTelegramMessage POSTs a single sendMessage call. Returns the message id,
// Telegram's ok flag, its error description, and any transport error.
func sendTelegramMessage(ctx context.Context, botToken string, payload map[string]interface{}) (int64, bool, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, false, "", fmt.Errorf("failed to marshal payload: %w", err)
	}
	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, false, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, false, "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(respBody, &result)
	return result.Result.MessageID, result.OK, result.Description, nil
}

// splitTelegramMessage splits text into chunks each within Telegram's 4096
// UTF-16-code-unit limit, breaking on the last newline (then space) before the
// limit so lines/words aren't cut mid-way. A single over-long token is
// hard-split on a rune boundary. Returns []{text} unchanged when it fits.
//
// Note: for parse_mode HTML/MarkdownV2 this splits on line boundaries, which is
// safe for typical content but could break a formatting entity that spans the
// split point — an accepted trade-off versus silent truncation.
func splitTelegramMessage(text string) []string {
	if utf16Len(text) <= maxMessageLen {
		return []string{text}
	}
	runes := []rune(text)
	var chunks []string
	for len(runes) > 0 {
		end, units := 0, 0
		for end < len(runes) {
			u := 1
			if runes[end] > 0xFFFF {
				u = 2
			}
			if units+u > maxMessageLen {
				break
			}
			units += u
			end++
		}
		// Prefer a clean boundary within the fitted prefix (unless everything
		// left already fits, in which case end == len(runes)).
		if end < len(runes) {
			if br := lastRuneIndex(runes[:end], '\n'); br > 0 {
				end = br + 1
			} else if br := lastRuneIndex(runes[:end], ' '); br > 0 {
				end = br + 1
			}
		}
		chunks = append(chunks, string(runes[:end]))
		runes = runes[end:]
	}
	return chunks
}

// utf16Len returns the length of s in UTF-16 code units (Telegram's unit).
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func lastRuneIndex(runes []rune, target rune) int {
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == target {
			return i
		}
	}
	return -1
}
