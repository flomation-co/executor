package ai_common

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	core "flomation.app/automate/executor"

	. "github.com/onsi/gomega"
)

// TestRecordAssistantReply_PostsOutboundWhenInAgentContext is the regression
// test for the conversation-loop fix. It proves that when an execution is
// running inside an agent orchestrator flow (ExecutionContext.AgentID set),
// the AI action's response is posted back to the API as an outbound
// agent_message. Without this, agent_message would contain only inbound user
// turns and the next turn's conversation_history would be a run of
// consecutive role:user messages — which causes the model to try to answer
// the entire history at once (the loop symptom).
func TestRecordAssistantReply_PostsOutboundWhenInAgentContext(t *testing.T) {
	RegisterTestingT(t)

	var (
		mu       sync.Mutex
		hitPaths []string
		received []map[string]interface{}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		hitPaths = append(hitPaths, r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		received = append(received, payload)

		// Route-specific status: the record endpoint expects 201,
		// the extract endpoint expects 202.
		if strings.HasSuffix(r.URL.Path, "/extract") {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"execution_id":"exec-1"}`))
		} else {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"msg-123"}`))
		}
	}))
	defer srv.Close()

	ctx := &core.ExecutionContext{
		AgentID: "agent-abc",
		APIURL:  srv.URL,
	}

	RecordAssistantReply(context.Background(), ctx, "Hello, this is the assistant's reply.")

	mu.Lock()
	defer mu.Unlock()

	// Phase 2d-γ: RecordAssistantReply now makes TWO calls — one to
	// record the outbound message, then one to dispatch extraction on
	// the assistant's text.
	Expect(hitPaths).To(HaveLen(2), "expected record + extraction calls")
	Expect(hitPaths[0]).To(Equal("/api/v1/internal/agent/agent-abc/message"),
		"first call must be the message record")
	Expect(hitPaths[1]).To(Equal("/api/v1/internal/agent/agent-abc/extract"),
		"second call must be the extraction dispatch")

	// Assert the record payload (first call).
	recordPayload := received[0]
	Expect(recordPayload["direction"]).To(Equal("outbound"))
	Expect(recordPayload["content"]).To(Equal("Hello, this is the assistant's reply."))
	Expect(recordPayload["sender"]).To(Equal("agent"))

	// Assert the extraction payload (second call).
	extractPayload := received[1]
	Expect(extractPayload["role"]).To(Equal("assistant"))
	Expect(extractPayload["content"]).To(Equal("Hello, this is the assistant's reply."))
}

// TestRecordAssistantReply_NoOpWhenNotInAgentContext verifies that a regular
// non-agent execution using an AI action does not attempt to record anything.
// This is the common case for flows that use OpenAI/Anthropic actions for
// non-conversational purposes (e.g. batch classification, one-shot inference)
// and should not touch the agent_message table.
func TestRecordAssistantReply_NoOpWhenNotInAgentContext(t *testing.T) {
	RegisterTestingT(t)

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Context with no AgentID — regular non-agent execution.
	ctx := &core.ExecutionContext{
		APIURL: srv.URL,
	}
	RecordAssistantReply(context.Background(), ctx, "some reply")
	Expect(called).To(BeFalse(), "no record call when AgentID is empty")

	// Nil context — defensive.
	RecordAssistantReply(context.Background(), nil, "some reply")
	Expect(called).To(BeFalse(), "no record call when context is nil")

	// Empty content — nothing to record.
	RecordAssistantReply(context.Background(), &core.ExecutionContext{
		AgentID: "agent-abc",
		APIURL:  srv.URL,
	}, "")
	Expect(called).To(BeFalse(), "no record call when content is empty")

	// Empty APIURL — test context without a reachable API.
	RecordAssistantReply(context.Background(), &core.ExecutionContext{
		AgentID: "agent-abc",
	}, "some reply")
	Expect(called).To(BeFalse(), "no record call when APIURL is empty")
}

// TestRecordAssistantReply_NonFatalOnAPIFailure verifies that a failed or
// slow recording call never panics or returns an error to the caller. The
// AI action has already returned its response to the caller at this point;
// a bookkeeping miss must not corrupt the user-visible output.
func TestRecordAssistantReply_NonFatalOnAPIFailure(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"db down"}`))
	}))
	defer srv.Close()

	ctx := &core.ExecutionContext{
		AgentID: "agent-abc",
		APIURL:  srv.URL,
	}

	// Should not panic.
	Expect(func() {
		RecordAssistantReply(context.Background(), ctx, "some reply")
	}).NotTo(Panic())
}

// TestExecutionContextGet_AgentID verifies the new agent_id field is
// reachable via the generic context getter, so ${flow.agent_id} in variable
// substitution resolves correctly for flow authors who want to reference it.
func TestExecutionContextGet_AgentID(t *testing.T) {
	RegisterTestingT(t)

	ctx := &core.ExecutionContext{AgentID: "agent-xyz"}
	Expect(ctx.Get("agent_id")).To(Equal("agent-xyz"))

	empty := &core.ExecutionContext{}
	Expect(empty.Get("agent_id")).To(Equal(""))
}

// Ensure the helper actually uses strings.Contains on the URL path, as a
// sanity check against regressions that break endpoint routing.
func TestRecordAssistantReply_URLContainsAgentID(t *testing.T) {
	RegisterTestingT(t)

	var mu sync.Mutex
	var urlPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		urlPaths = append(urlPaths, r.URL.Path)
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/extract") {
			w.WriteHeader(http.StatusAccepted)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()

	RecordAssistantReply(context.Background(), &core.ExecutionContext{
		AgentID: "special-agent-id",
		APIURL:  srv.URL,
	}, "content")

	mu.Lock()
	defer mu.Unlock()
	for _, p := range urlPaths {
		Expect(strings.Contains(p, "special-agent-id")).To(BeTrue())
	}
}

// --- Phase 2d-γ extraction dispatch tests ---

func TestDispatchAssistantExtraction_IncludesUserAndConversationIDs(t *testing.T) {
	RegisterTestingT(t)

	var (
		mu      sync.Mutex
		hitPath string
		body    map[string]interface{}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		hitPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	ctx := &core.ExecutionContext{
		AgentID:        "agent-1",
		APIURL:         srv.URL,
		AgentUserID:    "user-abc",
		ConversationID: "conv-xyz",
	}
	dispatchAssistantExtraction(context.Background(), ctx, "I'll come back to you.")

	mu.Lock()
	defer mu.Unlock()

	Expect(hitPath).To(Equal("/api/v1/internal/agent/agent-1/extract"))
	Expect(body["role"]).To(Equal("assistant"))
	Expect(body["content"]).To(Equal("I'll come back to you."))
	Expect(body["agent_user_id"]).To(Equal("user-abc"))
	Expect(body["conversation_id"]).To(Equal("conv-xyz"))
}

func TestDispatchAssistantExtraction_OmitsEmptyOptionals(t *testing.T) {
	RegisterTestingT(t)

	var (
		mu   sync.Mutex
		body map[string]interface{}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	ctx := &core.ExecutionContext{
		AgentID: "agent-1",
		APIURL:  srv.URL,
		// AgentUserID and ConversationID deliberately empty
	}
	dispatchAssistantExtraction(context.Background(), ctx, "some reply")

	mu.Lock()
	defer mu.Unlock()

	_, hasUser := body["agent_user_id"]
	_, hasConv := body["conversation_id"]
	Expect(hasUser).To(BeFalse(), "empty agent_user_id should not appear in payload")
	Expect(hasConv).To(BeFalse(), "empty conversation_id should not appear in payload")
}

func TestDispatchAssistantExtraction_NoOpWhenNotInAgentContext(t *testing.T) {
	RegisterTestingT(t)

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	// No AgentID → should not even make the call.
	dispatchAssistantExtraction(context.Background(), &core.ExecutionContext{
		APIURL: srv.URL,
	}, "content")
	Expect(called).To(BeFalse(), "no extraction call when AgentID is empty")

	// Nil context
	dispatchAssistantExtraction(context.Background(), nil, "content")
	Expect(called).To(BeFalse(), "no extraction call when context is nil")

	// Empty content
	dispatchAssistantExtraction(context.Background(), &core.ExecutionContext{
		AgentID: "agent-1", APIURL: srv.URL,
	}, "")
	Expect(called).To(BeFalse(), "no extraction call when content is empty")
}
