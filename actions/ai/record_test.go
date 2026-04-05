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
		received []map[string]interface{}
		hitPath  string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		hitPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		received = append(received, payload)

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"msg-123"}`))
	}))
	defer srv.Close()

	ctx := &core.ExecutionContext{
		AgentID: "agent-abc",
		APIURL:  srv.URL,
	}

	RecordAssistantReply(context.Background(), ctx, "Hello, this is the assistant's reply.")

	mu.Lock()
	defer mu.Unlock()

	Expect(received).To(HaveLen(1), "exactly one outbound record request should be made")
	Expect(hitPath).To(Equal("/api/v1/internal/agent/agent-abc/message"),
		"recording must POST to the agent's message endpoint")

	payload := received[0]
	Expect(payload["direction"]).To(Equal("outbound"))
	Expect(payload["content"]).To(Equal("Hello, this is the assistant's reply."))
	Expect(payload["sender"]).To(Equal("agent"))
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

	var urlPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	RecordAssistantReply(context.Background(), &core.ExecutionContext{
		AgentID: "special-agent-id",
		APIURL:  srv.URL,
	}, "content")

	Expect(strings.Contains(urlPath, "special-agent-id")).To(BeTrue())
}
