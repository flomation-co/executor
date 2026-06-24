package start

// Tests for the plan/start action. Mirrors plan/cancel + plan/block
// test pattern: stub httptest server + assertions on wire + outputs.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func newStartTestFlow(apiURL string) *core.Flow {
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{
		APIURL:  apiURL,
		AgentID: "agent-1",
	})
	return flow
}

func connsFromMap(values map[string]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(values))
	for k, v := range values {
		s := v
		out = append(out, &core.Connection{
			Name: k, Type: core.ConnectionTypeString, Value: s,
		})
	}
	return out
}

func TestExecute_HappyPath_ReturnsStarted(t *testing.T) {
	RegisterTestingT(t)
	var sawPath, sawMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_id":"plan-abc","outcome":"started"}`))
	}))
	defer srv.Close()

	flow := newStartTestFlow(srv.URL)
	out, err := Execute(flow, nil, connsFromMap(map[string]string{"plan_id": "plan-abc"}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["outcome"]).To(Equal("started"))
	Expect(out["tool_result"]).To(ContainSubstring("started"))
	Expect(sawMethod).To(Equal(http.MethodPost))
	Expect(sawPath).To(Equal("/api/v1/internal/agent/agent-1/plan/plan-abc/start"))
}

func TestExecute_IdempotentOutcome_StillSuccess(t *testing.T) {
	// Already-active plan returns idempotent — must NOT be a
	// failure from the AI's perspective.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_id":"plan-abc","outcome":"idempotent"}`))
	}))
	defer srv.Close()

	flow := newStartTestFlow(srv.URL)
	out, err := Execute(flow, nil, connsFromMap(map[string]string{"plan_id": "plan-abc"}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["outcome"]).To(Equal("idempotent"))
	Expect(out["tool_result"]).To(ContainSubstring("already active"))
}

func TestExecute_AlreadyTerminal_409_FailsClean(t *testing.T) {
	// Trying to start a cancelled/completed plan returns 409.
	// The AI MUST see this as a hard error and not retry — the
	// plan can't be resurrected.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"plan_already_terminal","detail":"plan is completed, cancelled, or blocked"}`))
	}))
	defer srv.Close()

	flow := newStartTestFlow(srv.URL)
	out, err := Execute(flow, nil, connsFromMap(map[string]string{"plan_id": "plan-abc"}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("already terminal"))
}

func TestExecute_NotFound_FailsClean(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	flow := newStartTestFlow(srv.URL)
	out, err := Execute(flow, nil, connsFromMap(map[string]string{"plan_id": "missing"}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("not found"))
}

func TestExecute_MissingPlanID_FailsClean(t *testing.T) {
	RegisterTestingT(t)
	flow := newStartTestFlow("http://unused")
	out, err := Execute(flow, nil, connsFromMap(map[string]string{"plan_id": ""}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("plan_id is required"))
}

func TestExecute_NoAPIURL_FailsFast(t *testing.T) {
	RegisterTestingT(t)
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{})
	out, err := Execute(flow, nil, connsFromMap(map[string]string{"plan_id": "plan-abc"}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("agent context"))
}

func TestExecute_ContextCancellation_PropagatesViaFlow(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	flow := newStartTestFlow(srv.URL)
	flow.SetCancelContext(ctx, cancel)
	cancel()
	out, err := Execute(flow, nil, connsFromMap(map[string]string{"plan_id": "plan-abc"}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
}
