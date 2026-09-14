package ai_common

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// The behaviour these pin down is a precedence rule, and getting it wrong in
// either direction is bad in a different way. Fetch when the flow supplied
// history and you override the author and double the API load. Fail to fetch
// when it did not and you get the bug this exists to fix: an agent that has
// forgotten everything, silently, with no error anywhere.

func agentContext(apiURL string) *core.ExecutionContext {
	return &core.ExecutionContext{
		AgentID:        "agent-1",
		ConversationID: "11111111-2222-3333-4444-555555555555",
		APIURL:         apiURL,
	}
}

// historyServer stands in for the API and counts requests, so a test can assert
// that no call was made as well as what came back.
func historyServer(t *testing.T, body string) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		RegisterTestingT(t)
		Expect(r.URL.Path).To(Equal("/api/v1/internal/conversation/11111111-2222-3333-4444-555555555555/history"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

const twoTurns = `[
  {"direction":"inbound","content":"what is the URL?"},
  {"direction":"outbound","content":"here it is"}
]`

// The case that was broken on live: the input exists but is blank, which used
// to mean "no history" and now means "fetch it".
func TestResolveConversationHistory_FetchesWhenTheInputIsBlank(t *testing.T) {
	RegisterTestingT(t)
	server, calls := historyServer(t, twoTurns)

	history := ResolveConversationHistory(context.Background(), agentContext(server.URL), "")

	Expect(*calls).To(Equal(1))
	Expect(history).To(Equal([]Message{
		{Role: "user", Content: "what is the URL?"},
		{Role: "assistant", Content: "here it is"},
	}))
}

// An unresolved reference is the same failure wearing a different hat — it is
// what a flow shows when the node it pointed at never ran.
func TestResolveConversationHistory_FetchesWhenTheReferenceDidNotResolve(t *testing.T) {
	RegisterTestingT(t)
	server, calls := historyServer(t, twoTurns)

	history := ResolveConversationHistory(context.Background(), agentContext(server.URL), "${conversation_history}")

	Expect(*calls).To(Equal(1))
	Expect(history).To(HaveLen(2))
}

func TestResolveConversationHistory_FetchesWhenTheInputIsAbsent(t *testing.T) {
	RegisterTestingT(t)
	server, calls := historyServer(t, twoTurns)

	history := ResolveConversationHistory(context.Background(), agentContext(server.URL), nil)

	Expect(*calls).To(Equal(1))
	Expect(history).To(HaveLen(2))
}

// Every flow that wires history today must behave exactly as it did.
func TestResolveConversationHistory_SuppliedInputWins(t *testing.T) {
	RegisterTestingT(t)
	server, calls := historyServer(t, twoTurns)

	supplied := []interface{}{
		map[string]interface{}{"role": "user", "content": "from the flow"},
	}
	history := ResolveConversationHistory(context.Background(), agentContext(server.URL), supplied)

	Expect(*calls).To(Equal(0), "a supplied history must not trigger a request")
	Expect(history).To(Equal([]Message{{Role: "user", Content: "from the flow"}}))
}

// The opt-out a long-running flow needs: it manages its own history, emitting
// several turns per execution, so an automatic fetch would fight it.
func TestResolveConversationHistory_AnExplicitlyEmptyListOptsOut(t *testing.T) {
	RegisterTestingT(t)
	server, calls := historyServer(t, twoTurns)

	history := ResolveConversationHistory(context.Background(), agentContext(server.URL), []interface{}{})

	Expect(*calls).To(Equal(0), "an explicit empty list is an answer, not an absence")
	Expect(history).To(BeEmpty())
}

// Outside an agent context nothing should be fetched — an ordinary flow that
// happens to use an AI action has no conversation to read.
func TestFetchConversationHistory_NoOpsOutsideAnAgentContext(t *testing.T) {
	RegisterTestingT(t)
	server, calls := historyServer(t, twoTurns)

	Expect(FetchConversationHistory(context.Background(), nil, 30)).To(BeNil())

	noAgent := agentContext(server.URL)
	noAgent.AgentID = ""
	Expect(FetchConversationHistory(context.Background(), noAgent, 30)).To(BeNil())

	// An agent turn with no conversation — a schedule-triggered orchestrator
	// rather than a channel message.
	noConversation := agentContext(server.URL)
	noConversation.ConversationID = ""
	Expect(FetchConversationHistory(context.Background(), noConversation, 30)).To(BeNil())

	noAPI := agentContext("")
	Expect(FetchConversationHistory(context.Background(), noAPI, 30)).To(BeNil())

	Expect(*calls).To(Equal(0))
}

// A failure must cost the turn its history, never the answer. The previous
// behaviour was no history in every case, so degrading to that is not a
// regression — failing the action would be.
func TestFetchConversationHistory_FailureDegradesToNoHistory(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	Expect(FetchConversationHistory(context.Background(), agentContext(server.URL), 30)).To(BeNil())

	// Malformed JSON likewise.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer bad.Close()
	Expect(FetchConversationHistory(context.Background(), agentContext(bad.URL), 30)).To(BeNil())
}

// Tool exchanges share the agent_message table but are not conversation turns.
// Presenting them as such would show the model tool chatter as if the user had
// said it.
func TestTransformHistory_SkipsToolExchangesAndEmptyContent(t *testing.T) {
	RegisterTestingT(t)

	body := []byte(`[
	  {"direction":"inbound","content":"hello"},
	  {"direction":"tool_use","content":"web/fetch(url=...)"},
	  {"direction":"tool_result","content":"<html>..."},
	  {"direction":"outbound","content":""},
	  {"direction":"outbound","content":"hi there"}
	]`)

	Expect(transformHistory(body)).To(Equal([]Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}))
}

// The mapping must match the write path's vocabulary, or replies come back
// labelled as though the user said them.
func TestTransformHistory_MapsDirectionToRole(t *testing.T) {
	RegisterTestingT(t)

	body := []byte(`[{"direction":"inbound","content":"a"},{"direction":"outbound","content":"b"},{"direction":"","content":"c"}]`)
	messages := transformHistory(body)

	Expect(messages[0].Role).To(Equal("user"))
	Expect(messages[1].Role).To(Equal("assistant"))
	// Anything that is not explicitly outbound is a user turn; treating an
	// unknown direction as assistant would put words in the model's mouth.
	Expect(messages[2].Role).To(Equal("user"))
}

func TestConversationHistoryFor_HandlesANilConnection(t *testing.T) {
	RegisterTestingT(t)

	// No flow, no context — the unit-test shape. Must not panic and must not
	// invent history.
	Expect(ConversationHistoryFor(nil, nil)).To(BeEmpty())
}
