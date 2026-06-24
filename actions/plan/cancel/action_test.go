package cancel

// Tests for the plan/cancel action. Mirrors the plan/block test
// pattern: a stub httptest server stands in for the API; tests
// assert (a) what the action sent on the wire and (b) the outputs
// the action returned to the flow engine.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func newCancelTestFlow(apiURL string) *core.Flow {
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{
		APIURL:      apiURL,
		AgentID:     "agent-1",
		ExecutionID: "exec-current",
	})
	return flow
}

func connsFromMap(values map[string]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(values))
	for k, v := range values {
		s := v
		out = append(out, &core.Connection{
			Name:  k,
			Type:  core.ConnectionTypeString,
			Value: s,
		})
	}
	return out
}

func TestExecute_HappyPath_ReturnsCancelled(t *testing.T) {
	RegisterTestingT(t)

	var sawPath string
	var sawBody struct {
		Reason string `json:"reason"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		sawPath = r.URL.Path
		Expect(json.NewDecoder(r.Body).Decode(&sawBody)).To(Succeed())

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_id":"plan-abc","outcome":"cancelled"}`))
	}))
	defer srv.Close()

	flow := newCancelTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{
		"plan_id": "plan-abc",
		"reason":  "user changed their mind",
	})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["outcome"]).To(Equal("cancelled"))
	Expect(out["plan_id"]).To(Equal("plan-abc"))
	Expect(out["tool_result"]).To(ContainSubstring("cancelled"))

	Expect(sawPath).To(Equal("/api/v1/internal/agent/agent-1/plan/plan-abc/cancel"))
	Expect(sawBody.Reason).To(Equal("user changed their mind"))
}

func TestExecute_IdempotentOutcome_StillSuccess(t *testing.T) {
	// Cancelling an already-terminal plan returns idempotent. The
	// AI should NOT see this as a failure — calling cancel twice
	// (or cancelling a completed plan) is benign.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_id":"plan-abc","outcome":"idempotent"}`))
	}))
	defer srv.Close()

	flow := newCancelTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{"plan_id": "plan-abc"})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["outcome"]).To(Equal("idempotent"))
	Expect(out["tool_result"]).To(ContainSubstring("already terminal"))
}

func TestExecute_NotFound_SurfacesAsFailedToolResult(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"plan_not_found","plan_id":"plan-missing"}`))
	}))
	defer srv.Close()

	flow := newCancelTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{"plan_id": "plan-missing"})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("not found"))
}

func TestExecute_MissingPlanID_FailsClean(t *testing.T) {
	// plan_id is mandatory — without it we'd hit /plan//cancel
	// which is a routing failure on the API side. Catch it early.
	RegisterTestingT(t)
	flow := newCancelTestFlow("http://unused")
	inputs := connsFromMap(map[string]string{"plan_id": "", "reason": "x"})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("plan_id is required"))
}

func TestExecute_NoBody_StillSendsEmptyReason(t *testing.T) {
	// The AI may invoke cancel without typing a reason. The body
	// must still be valid JSON ({"reason":""}) so the API's
	// JSON binding doesn't 400.
	RegisterTestingT(t)
	var sawBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 200)
		n, _ := r.Body.Read(buf)
		sawBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_id":"plan-abc","outcome":"cancelled"}`))
	}))
	defer srv.Close()

	flow := newCancelTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{"plan_id": "plan-abc"})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(sawBody).To(ContainSubstring(`"reason":""`))
}

func TestExecute_NoAPIURL_FailsFast(t *testing.T) {
	RegisterTestingT(t)
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{}) // no APIURL, no AgentID
	inputs := connsFromMap(map[string]string{"plan_id": "plan-abc"})

	out, err := Execute(flow, nil, inputs)
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
	flow := newCancelTestFlow(srv.URL)
	flow.SetCancelContext(ctx, cancel)
	cancel()
	inputs := connsFromMap(map[string]string{"plan_id": "plan-abc"})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
}
