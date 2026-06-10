package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// messagingActionToChannel maps a tool node's label to the canonical
// channel_type the API expects when recording an outbound relay.
//
// The mapping is hand-maintained here rather than derived from the
// action package because:
//
//  1. Action developers don't need to know about conversation
//     semantics — the user explicitly asked for "logic in the thing
//     that cares about conversation", not in the actions.
//  2. Plumbing a category flag through the manifest generator was
//     considered and dropped as over-engineering for a single
//     additional concept. If the list grows past ~20 entries or new
//     channels start landing weekly, revisit.
//  3. New messaging categories are rare events; adding one line here
//     when shipping a new channel is a known checkpoint, not a hidden
//     coupling.
//
// Labels match what the manifest's action `Type` constants resolve
// to — see actions/slack/send_message/action.go and friends.
var messagingActionToChannel = map[string]string{
	"slack/send_message":              "slack",
	"slack/rich_message":              "slack",
	"slack/file_upload":               "slack",
	"messaging/telegram/send_message": "telegram",
	"messaging/telegram/send_voice":   "telegram",
	"messaging/discord/send_message":  "discord",
	"messaging/email/send":            "email",
}

// recordOutboundPayload is the wire format for
// POST /api/v1/internal/agent/:id/record-outbound.
type recordOutboundPayload struct {
	ChannelType          string  `json:"channel_type"`
	ChannelID            string  `json:"channel_id"`
	RecipientID          string  `json:"recipient_id,omitempty"`
	Content              string  `json:"content"`
	SourceConversationID *string `json:"source_conversation_id,omitempty"`
}

// recordOutboundRelay is invoked by the AI tool dispatch loop after a
// matched tool node executes successfully. If the node's label
// identifies it as a messaging action, the hook resolves recipient +
// content from the node's inputs and POSTs to the API's
// record-outbound endpoint.
//
// Failure modes that resolve to silent no-ops (logged at debug):
//
//   - Not running in agent context (ctx.AgentID empty): conversation
//     recording is an agent-only concept, no agent = nothing to
//     record.
//   - Label isn't a known messaging action: nothing to do.
//   - Missing channel_id / content inputs: the action presumably
//     produced its own error; don't compound.
//
// Failure modes that resolve to logged warnings:
//
//   - API call fails / non-2xx: the message was already sent
//     successfully by the action; we just couldn't record it. The
//     send is the load-bearing operation; the record is bookkeeping.
func (f *Flow) recordOutboundRelay(matchedTool *Node) {
	if matchedTool == nil || matchedTool.Data == nil {
		return
	}
	ctx := f.GetContext()
	if ctx == nil || ctx.AgentID == "" {
		// Not an agent execution — nothing to record against.
		return
	}

	channelType, ok := messagingActionToChannel[matchedTool.Data.Label]
	if !ok {
		// Not a messaging tool — nothing to do.
		return
	}

	channelID := firstNonEmptyInput(matchedTool, "channel_id", "chat_id", "recipient")
	if channelID == "" {
		log.WithFields(log.Fields{
			"agent_id": ctx.AgentID,
			"label":    matchedTool.Data.Label,
		}).Debug("record-outbound: matched tool produced no channel id; skipping")
		return
	}
	content := firstNonEmptyInput(matchedTool, "message", "content", "text")
	if content == "" {
		log.WithFields(log.Fields{
			"agent_id":   ctx.AgentID,
			"label":      matchedTool.Data.Label,
			"channel_id": channelID,
		}).Debug("record-outbound: matched tool produced no message body; skipping")
		return
	}

	// For Telegram DMs the chat_id IS the recipient's stable
	// numeric sender id — same value, used both as the conversation
	// scoping key and the identity-resolution external_id. Slack DMs
	// follow the same convention (the destination U-id IS the
	// recipient).
	//
	// The API endpoint treats RecipientID as the optional secondary
	// identifier used for declared-identity lookup, so a value here
	// just upgrades the conversation from channel-scoped to
	// user-scoped when a matching user_identity exists.
	recipientID := channelID

	payload := recordOutboundPayload{
		ChannelType: channelType,
		ChannelID:   channelID,
		RecipientID: recipientID,
		Content:     content,
	}
	if ctx.ConversationID != "" {
		conv := ctx.ConversationID
		payload.SourceConversationID = &conv
	}

	if err := postRecordOutbound(ctx, payload); err != nil {
		log.WithFields(log.Fields{
			"agent_id":     ctx.AgentID,
			"channel_type": channelType,
			"channel_id":   channelID,
			"error":        err,
		}).Warn("record-outbound: API call failed; send already succeeded but relay won't be in conversation history")
	}
}

// firstNonEmptyInput returns the value of the first input whose name
// matches one of `names` and whose string value is non-empty. Used to
// support actions that have settled on different conventional input
// names ("channel_id" vs "chat_id", "message" vs "content").
func firstNonEmptyInput(node *Node, names ...string) string {
	if node == nil || node.Data == nil {
		return ""
	}
	for _, name := range names {
		c := FindConnection(name, node.Data.Config.Inputs)
		if c == nil {
			continue
		}
		s := c.String()
		if s == nil || *s == "" {
			continue
		}
		return *s
	}
	return ""
}

// postRecordOutbound is the HTTP plumbing. Uses the execution context's
// API URL + mTLS-aware InternalClient — same pattern as every other
// internal API call the executor makes.
func postRecordOutbound(ctx *ExecutionContext, payload recordOutboundPayload) error {
	if ctx == nil || ctx.APIURL == "" {
		return fmt.Errorf("missing API url in execution context")
	}

	url := fmt.Sprintf("%s/api/v1/internal/agent/%s/record-outbound", ctx.APIURL, ctx.AgentID)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	client := ctx.InternalClient()
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("api call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("api returned %d", resp.StatusCode)
	}
	return nil
}
