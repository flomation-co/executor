package await

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Option is a single choice presented to the human. Value drives the output
// handle ("option_<value>"); Label is what the human sees; Token is the
// per-option capability minted by the API for the web click-link fallback.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Token string `json:"token"`
}

// ActionIDPrefix tags Slack button action_ids and Telegram callback_data so
// the Launch service can distinguish a Human-in-the-Loop response from an
// ordinary agent interaction. Kept in sync with launch's hitl handler.
const ActionIDPrefix = "hitl:"

// RenderSlackBlocks builds a Block Kit payload: a section with the prompt
// followed by an actions block of buttons, one per option. Each button's
// action_id encodes the request and option so Launch can resolve them
// (Slack action_ids are limited to 255 chars — request_id + value fit easily).
func RenderSlackBlocks(message, requestID string, opts []Option) json.RawMessage {
	type txt struct {
		Type  string `json:"type"`
		Text  string `json:"text"`
		Emoji *bool  `json:"emoji,omitempty"`
	}
	type button struct {
		Type     string `json:"type"`
		Text     txt    `json:"text"`
		ActionID string `json:"action_id"`
		Value    string `json:"value"`
	}
	type block struct {
		Type     string        `json:"type"`
		Text     *txt          `json:"text,omitempty"`
		Elements []interface{} `json:"elements,omitempty"`
	}

	emoji := true
	elements := make([]interface{}, 0, len(opts))
	for _, o := range opts {
		elements = append(elements, button{
			Type:     "button",
			Text:     txt{Type: "plain_text", Text: o.Label, Emoji: &emoji},
			ActionID: ActionIDPrefix + requestID + ":" + o.Value,
			Value:    o.Value,
		})
	}

	blocks := []block{
		{Type: "section", Text: &txt{Type: "mrkdwn", Text: message}},
		{Type: "actions", Elements: elements},
	}
	b, _ := json.Marshal(blocks)
	return b
}

// RenderTelegramKeyboard builds an inline_keyboard reply_markup, one button
// per option. Telegram limits callback_data to 64 bytes, so we send only the
// short per-option token ("hitl:" + token) rather than the request id + value.
func RenderTelegramKeyboard(opts []Option) json.RawMessage {
	type ikb struct {
		Text         string `json:"text"`
		CallbackData string `json:"callback_data"`
	}
	rows := make([][]ikb, 0, len(opts))
	for _, o := range opts {
		rows = append(rows, []ikb{{
			Text:         o.Label,
			CallbackData: ActionIDPrefix + o.Token,
		}})
	}
	markup := map[string]interface{}{"inline_keyboard": rows}
	b, _ := json.Marshal(markup)
	return b
}

// RenderWebLinks returns the message with one tokenised click-link appended
// per option. This is the channel-agnostic fallback used by email, SMS,
// Discord and any channel without native interactive buttons.
func RenderWebLinks(message, baseURL string, opts []Option) string {
	baseURL = strings.TrimRight(baseURL, "/")
	var b strings.Builder
	b.WriteString(message)
	b.WriteString("\n")
	for _, o := range opts {
		b.WriteString(fmt.Sprintf("\n%s: %s/respond/%s", o.Label, baseURL, o.Token))
	}
	return b.String()
}
