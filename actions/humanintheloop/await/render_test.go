package await

import (
	"encoding/json"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

func TestRenderSlackBlocks_ButtonsEncodeRequestAndOption(t *testing.T) {
	opts := []Option{{Value: "yes", Label: "Approve", Token: "t1"}, {Value: "no", Label: "Deny", Token: "t2"}}
	raw := RenderSlackBlocks("Ship it?", "req-9", opts)

	var blocks []map[string]interface{}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		t.Fatalf("blocks are not valid JSON: %v", err)
	}
	if len(blocks) != 2 || blocks[0]["type"] != "section" || blocks[1]["type"] != "actions" {
		t.Fatalf("unexpected block structure: %s", raw)
	}
	elements, _ := blocks[1]["elements"].([]interface{})
	if len(elements) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(elements))
	}
	first, _ := elements[0].(map[string]interface{})
	if first["action_id"] != "hitl:req-9:yes" {
		t.Errorf("action_id = %v, want hitl:req-9:yes", first["action_id"])
	}
	if first["value"] != "yes" {
		t.Errorf("value = %v, want yes", first["value"])
	}
}

func TestRenderTelegramKeyboard_CallbackDataWithinLimit(t *testing.T) {
	opts := []Option{{Value: "approve", Label: "Approve", Token: "abcDEF123456"}}
	raw := RenderTelegramKeyboard(opts)

	var markup struct {
		InlineKeyboard [][]struct {
			Text         string `json:"text"`
			CallbackData string `json:"callback_data"`
		} `json:"inline_keyboard"`
	}
	if err := json.Unmarshal(raw, &markup); err != nil {
		t.Fatalf("reply_markup is not valid JSON: %v", err)
	}
	if len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
		t.Fatalf("unexpected keyboard shape: %s", raw)
	}
	btn := markup.InlineKeyboard[0][0]
	if btn.CallbackData != "hitl:abcDEF123456" {
		t.Errorf("callback_data = %q", btn.CallbackData)
	}
	if len(btn.CallbackData) > 64 {
		t.Errorf("callback_data exceeds Telegram's 64-byte limit: %d", len(btn.CallbackData))
	}
}

func TestRenderWebLinks_AppendsTokenisedLinkPerOption(t *testing.T) {
	opts := []Option{{Value: "yes", Label: "Approve", Token: "tok1"}, {Value: "no", Label: "Deny", Token: "tok2"}}
	out := RenderWebLinks("Ship it?", "https://launch.example.com/", opts)

	if !strings.Contains(out, "Ship it?") {
		t.Error("message body missing")
	}
	if !strings.Contains(out, "Approve: https://launch.example.com/respond/tok1") {
		t.Errorf("approve link missing or malformed:\n%s", out)
	}
	if !strings.Contains(out, "Deny: https://launch.example.com/respond/tok2") {
		t.Errorf("deny link missing or malformed:\n%s", out)
	}
	if strings.Contains(out, "//respond") {
		t.Error("trailing slash on base URL produced a double slash")
	}
}

func TestParseOptions_DerivesValueFromLabelWhenBlank(t *testing.T) {
	optionsConn := &core.Connection{
		Name:  "options",
		Type:  core.ConnectionTypeKeyValueArray,
		Value: `[{"key":"Approve Deploy","value":""},{"key":"Reject","value":"no"}]`,
	}
	opts := parseOptions([]*core.Connection{optionsConn})
	if len(opts) != 2 {
		t.Fatalf("expected 2 options, got %d", len(opts))
	}
	if opts[0].Value != "approve_deploy" {
		t.Errorf("derived value = %q, want approve_deploy", opts[0].Value)
	}
	if opts[1].Value != "no" || opts[1].Label != "Reject" {
		t.Errorf("second option = %+v", opts[1])
	}
}

func TestInferChannel(t *testing.T) {
	cases := map[string]string{
		"slack/send_message":              "slack",
		"messaging/telegram/send_message": "telegram",
		"messaging/email/send":            "email",
		"messaging/discord/webhook":       "discord",
		"twilio/send_sms":                 "sms",
		"some/unknown":                    "",
	}
	for label, want := range cases {
		if got := inferChannel(label, ""); got != want {
			t.Errorf("inferChannel(%q) = %q, want %q", label, got, want)
		}
	}
}
