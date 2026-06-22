package block

// Tests for the plan/block action. Mirrors the plan/create test
// pattern: a stub httptest server stands in for the API, and the
// tests assert on (a) what the action sent to the wire and (b) the
// outputs the action returned to the flow engine.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func newBlockTestFlow(apiURL string) *core.Flow {
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{
		APIURL:      apiURL,
		AgentID:     "agent-1",
		ExecutionID: "exec-current",
		PlanTaskID:  "task-abc",
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

func TestExecute_HappyPath_ReturnsBlocked(t *testing.T) {
	// Confirms the URL shape (POST /api/v1/internal/plan_task/:id/block),
	// the request body (just reason), and that the outputs map
	// faithful API response data to the flow engine.
	RegisterTestingT(t)

	var sawPath string
	var sawBody struct {
		Reason string `json:"reason"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		sawPath = r.URL.Path
		Expect(json.NewDecoder(r.Body).Decode(&sawBody)).To(Succeed())

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_task_id":"task-abc","outcome":"blocked"}`))
	}))
	defer srv.Close()

	flow := newBlockTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{
		"plan_task_id": "task-abc",
		"reason":       "Missing BigQuery credentials in this environment",
	})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["outcome"]).To(Equal("blocked"))
	Expect(out["plan_task_id"]).To(Equal("task-abc"))
	Expect(out["tool_result"]).To(ContainSubstring("blocked"))
	Expect(out["tool_result"]).To(ContainSubstring("Missing BigQuery"))

	Expect(sawPath).To(Equal("/api/v1/internal/plan_task/task-abc/block"))
	Expect(sawBody.Reason).To(Equal("Missing BigQuery credentials in this environment"))
}

func TestExecute_IdempotentOutcome_StillSuccess(t *testing.T) {
	// Calling plan/block on an already-terminal task is a no-op
	// surfaced as outcome=idempotent. The AI shouldn't see this as a
	// failure — repeats across runner retries are expected.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_task_id":"task-abc","outcome":"idempotent"}`))
	}))
	defer srv.Close()

	flow := newBlockTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{
		"plan_task_id": "task-abc",
		"reason":       "giving up",
	})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["outcome"]).To(Equal("idempotent"))
}

func TestExecute_NotFound_SurfacesAsFailedToolResult(t *testing.T) {
	// 404 means the AI passed a stale or made-up task ID. The action
	// renders a clean tool_result so the model can read "not found"
	// and either retry on the next tick with the right ID or call
	// set_output to terminate.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"plan_task_not_found","plan_task_id":"task-missing"}`))
	}))
	defer srv.Close()

	flow := newBlockTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{
		"plan_task_id": "task-missing",
		"reason":       "x",
	})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("not found"))
	Expect(out["tool_result"]).To(ContainSubstring("404"))
}

func TestExecute_MissingPlanTaskID_FailsClean(t *testing.T) {
	// plan_task_id is mandatory. The placeholder ${flow.plan_task_id}
	// resolves at the engine layer — if it resolves empty (non-plan
	// context), we surface a clear tool_result instead of hitting
	// the API with /plan_task//block.
	RegisterTestingT(t)
	flow := newBlockTestFlow("http://unused")
	inputs := connsFromMap(map[string]string{
		"plan_task_id": "",
		"reason":       "x",
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("plan_task_id is required"))
}

func TestExecute_MissingReason_FailsClean(t *testing.T) {
	// reason is the whole point of plan/block — without one the
	// blocked state carries no signal a human can act on.
	RegisterTestingT(t)
	flow := newBlockTestFlow("http://unused")
	inputs := connsFromMap(map[string]string{
		"plan_task_id": "task-abc",
		"reason":       "",
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("reason is required"))
}

func TestExecute_NoAPIURL_FailsFast(t *testing.T) {
	// Defensive: the action only runs in an agent context where the
	// executor injects APIURL. A missing URL means we'd silently 404
	// against http:///... — fail clearly instead.
	RegisterTestingT(t)
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{}) // no APIURL
	inputs := connsFromMap(map[string]string{
		"plan_task_id": "task-abc",
		"reason":       "x",
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("agent context"))
}

func TestExecute_500Response_SurfacesStatusAndBody(t *testing.T) {
	// Unexpected API errors should surface verbatim (truncated to a
	// sane length) so the model can decide whether to retry or
	// terminate.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`internal error`))
	}))
	defer srv.Close()

	flow := newBlockTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{
		"plan_task_id": "task-abc",
		"reason":       "x",
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("500"))
	Expect(out["tool_result"]).To(ContainSubstring("internal error"))
}

func TestExecute_ContextCancellation_PropagatesViaFlow(t *testing.T) {
	// Cancelled flow context aborts the call without hanging.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	flow := newBlockTestFlow(srv.URL)
	flow.SetCancelContext(ctx, cancel)
	cancel()
	inputs := connsFromMap(map[string]string{
		"plan_task_id": "task-abc",
		"reason":       "x",
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).NotTo(BeNil())
}
