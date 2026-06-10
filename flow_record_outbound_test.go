package core

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// Hook decisions are visible in two places: which (channel_type,
// channel_id, content) the engine derives from a node's resolved
// inputs, and whether the HTTP call is fired at all. The HTTP plumbing
// is exercised end-to-end via httptest so any future drift between the
// payload shape here and the API handler's expected shape gets caught.

func TestRecordOutboundRelay_HappyPath_PostsToAPI(t *testing.T) {
	RegisterTestingT(t)

	var received recordOutboundPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.URL.Path).To(Equal("/api/v1/internal/agent/agent-1/record-outbound"))
		body, _ := io.ReadAll(r.Body)
		Expect(json.Unmarshal(body, &received)).To(Succeed())
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	flow := &Flow{}
	flow.SetContext(&ExecutionContext{
		AgentID:        "agent-1",
		ConversationID: "conv-123",
		APIURL:         srv.URL,
	})

	tool := &Node{
		ID:   "n1",
		Type: "slack/send_message",
		Data: &NodeData{
			ID:    "n1",
			Label: "slack/send_message",
			Config: NodeConfig{
				Inputs: []*Connection{
					{Name: "channel_id", Type: ConnectionTypeString, Value: "U_BOB"},
					{Name: "message", Type: ConnectionTypeString, Value: "hi from Andy"},
				},
			},
		},
	}

	flow.recordOutboundRelay(tool)

	Expect(received.ChannelType).To(Equal("slack"))
	Expect(received.ChannelID).To(Equal("U_BOB"))
	Expect(received.RecipientID).To(Equal("U_BOB"))
	Expect(received.Content).To(Equal("hi from Andy"))
	Expect(received.SourceConversationID).NotTo(BeNil())
	Expect(*received.SourceConversationID).To(Equal("conv-123"))
}

func TestRecordOutboundRelay_LegacyChatIdInput_StillResolved(t *testing.T) {
	// Telegram + a handful of older slack actions use "chat_id" rather
	// than the canonical "channel_id". The hook must accept both so the
	// hook doesn't need to wait for an action-side rename pass.
	RegisterTestingT(t)

	var received recordOutboundPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	flow := &Flow{}
	flow.SetContext(&ExecutionContext{AgentID: "agent-1", APIURL: srv.URL})

	tool := &Node{
		Data: &NodeData{
			Label: "messaging/telegram/send_message",
			Config: NodeConfig{
				Inputs: []*Connection{
					{Name: "chat_id", Type: ConnectionTypeString, Value: "7898807944"},
					{Name: "message", Type: ConnectionTypeString, Value: "remember to drink water"},
				},
			},
		},
	}

	flow.recordOutboundRelay(tool)

	Expect(received.ChannelType).To(Equal("telegram"))
	Expect(received.ChannelID).To(Equal("7898807944"))
	Expect(received.Content).To(Equal("remember to drink water"))
}

func TestRecordOutboundRelay_NotAMessagingAction_SkipsHTTPCall(t *testing.T) {
	// The hook must be a strict no-op for non-messaging tools.
	// Otherwise an http/get tool call would fire a meaningless
	// record-outbound request every time the AI hits the web.
	RegisterTestingT(t)

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	flow := &Flow{}
	flow.SetContext(&ExecutionContext{AgentID: "agent-1", APIURL: srv.URL})

	tool := &Node{
		Data: &NodeData{
			Label: "http/get",
			Config: NodeConfig{
				Inputs: []*Connection{
					{Name: "url", Type: ConnectionTypeString, Value: "https://example.com"},
				},
			},
		},
	}

	flow.recordOutboundRelay(tool)
	Expect(called).To(BeFalse())
}

func TestRecordOutboundRelay_NoAgentContext_SkipsHTTPCall(t *testing.T) {
	// Non-agent executions (manual flow runs, schedule triggers
	// without orchestrator context) don't have an agent_id. The hook
	// must skip silently — conversation recording requires an agent.
	RegisterTestingT(t)

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	flow := &Flow{}
	flow.SetContext(&ExecutionContext{APIURL: srv.URL}) // no AgentID

	tool := &Node{
		Data: &NodeData{
			Label: "slack/send_message",
			Config: NodeConfig{
				Inputs: []*Connection{
					{Name: "channel_id", Type: ConnectionTypeString, Value: "U_BOB"},
					{Name: "message", Type: ConnectionTypeString, Value: "hi"},
				},
			},
		},
	}

	flow.recordOutboundRelay(tool)
	Expect(called).To(BeFalse())
}

func TestRecordOutboundRelay_MissingContent_SkipsHTTPCall(t *testing.T) {
	// If the action somehow ran successfully but produced no message
	// body (e.g. the AI hallucinated an empty `message` field), there's
	// no useful conversation entry to write. Don't post junk.
	RegisterTestingT(t)

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	flow := &Flow{}
	flow.SetContext(&ExecutionContext{AgentID: "agent-1", APIURL: srv.URL})

	tool := &Node{
		Data: &NodeData{
			Label: "slack/send_message",
			Config: NodeConfig{
				Inputs: []*Connection{
					{Name: "channel_id", Type: ConnectionTypeString, Value: "U_BOB"},
					{Name: "message", Type: ConnectionTypeString, Value: ""},
				},
			},
		},
	}

	flow.recordOutboundRelay(tool)
	Expect(called).To(BeFalse())
}

func TestRecordOutboundRelay_NoConversationID_OmitsSourcePointer(t *testing.T) {
	// When the engine fires a tool outside of any conversation
	// (e.g. a schedule trigger whose flow has no inbound peer), the
	// recorded outbound has no source_conversation_id. The hook should
	// emit a payload without that field rather than e.g. an empty
	// string.
	RegisterTestingT(t)

	var rawBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rawBody = string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	flow := &Flow{}
	flow.SetContext(&ExecutionContext{AgentID: "agent-1", APIURL: srv.URL})

	tool := &Node{
		Data: &NodeData{
			Label: "slack/send_message",
			Config: NodeConfig{
				Inputs: []*Connection{
					{Name: "channel_id", Type: ConnectionTypeString, Value: "U_BOB"},
					{Name: "message", Type: ConnectionTypeString, Value: "hi"},
				},
			},
		},
	}

	flow.recordOutboundRelay(tool)

	Expect(strings.Contains(rawBody, "source_conversation_id")).To(BeFalse())
}
