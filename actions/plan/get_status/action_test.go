package get_status

// Tests for the plan/get_status action. Same pattern as plan/cancel:
// httptest stub server returns canned responses; tests assert on
// what was sent + what was returned.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func newStatusTestFlow(apiURL string) *core.Flow {
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
			Name:  k,
			Type:  core.ConnectionTypeString,
			Value: s,
		})
	}
	return out
}

func TestExecute_HappyPath_ReturnsHistogram(t *testing.T) {
	RegisterTestingT(t)

	var sawPath, sawMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"plan": {"id":"plan-abc","title":"Q3 review","status":"active"},
			"tasks": [
				{"id":"t1","name":"discover","status":"completed"},
				{"id":"t2","name":"expand","status":"in_progress"},
				{"id":"t3","name":"persist","status":"pending"}
			]
		}`))
	}))
	defer srv.Close()

	flow := newStatusTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{"plan_id": "plan-abc"})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["plan_id"]).To(Equal("plan-abc"))
	Expect(out["status"]).To(Equal("active"))
	Expect(out["task_count"]).To(Equal(3))
	Expect(out["completed_count"]).To(Equal(1))
	Expect(out["failed_count"]).To(Equal(0))
	Expect(out["tool_result"]).To(ContainSubstring("Q3 review"))
	Expect(out["tool_result"]).To(ContainSubstring("1/3"))

	Expect(sawMethod).To(Equal(http.MethodGet))
	Expect(sawPath).To(Equal("/api/v1/internal/agent/agent-1/plan/plan-abc"))
}

func TestExecute_FailedAndCancelled_RollIntoFailedCount(t *testing.T) {
	// failed_count includes both 'failed' and 'cancelled' tasks —
	// from the AI's "this task will not complete" perspective they
	// mean the same thing.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"plan": {"id":"plan-abc","title":"x","status":"blocked"},
			"tasks": [
				{"id":"t1","name":"a","status":"completed"},
				{"id":"t2","name":"b","status":"failed"},
				{"id":"t3","name":"c","status":"cancelled"}
			]
		}`))
	}))
	defer srv.Close()

	flow := newStatusTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{"plan_id": "plan-abc"})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["failed_count"]).To(Equal(2))
	Expect(out["tool_result"]).To(ContainSubstring("2 failed"))
}

func TestExecute_NotFound_FailsClean(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	flow := newStatusTestFlow(srv.URL)
	inputs := connsFromMap(map[string]string{"plan_id": "missing"})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("not found"))
}

func TestExecute_MissingPlanID_FailsClean(t *testing.T) {
	RegisterTestingT(t)
	flow := newStatusTestFlow("http://unused")
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
	flow := newStatusTestFlow(srv.URL)
	flow.SetCancelContext(ctx, cancel)
	cancel()
	out, err := Execute(flow, nil, connsFromMap(map[string]string{"plan_id": "plan-abc"}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
}
