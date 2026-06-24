package revise

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func newReviseTestFlow(apiURL string) *core.Flow {
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

func TestExecute_HappyPath_AddTask(t *testing.T) {
	RegisterTestingT(t)
	var sawPath string
	var sawBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		Expect(json.NewDecoder(r.Body).Decode(&sawBody)).To(Succeed())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_id":"plan-abc","outcome":"revised","new_status":"draft","added_ids":["new-id"]}`))
	}))
	defer srv.Close()

	flow := newReviseTestFlow(srv.URL)
	out, err := Execute(flow, nil, connsFromMap(map[string]string{
		"plan_id":   "plan-abc",
		"add_tasks": `[{"name":"new","description":"do it"}]`,
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["outcome"]).To(Equal("revised"))
	Expect(out["added_count"]).To(Equal(1))
	Expect(sawPath).To(Equal("/api/v1/internal/agent/agent-1/plan/plan-abc/revise"))
	Expect(sawBody["add_tasks"]).NotTo(BeNil())
}

func TestExecute_HappyPath_RemoveTask(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"plan_id":"plan-abc","outcome":"revised","new_status":"active","removed_ids":["task-id"]}`))
	}))
	defer srv.Close()

	flow := newReviseTestFlow(srv.URL)
	out, err := Execute(flow, nil, connsFromMap(map[string]string{
		"plan_id":      "plan-abc",
		"remove_tasks": `["old_task"]`,
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["removed_count"]).To(Equal(1))
	// new_status="active" suggests a blocked plan unblocked after
	// removing its failed task.
	Expect(out["new_status"]).To(Equal("active"))
}

func TestExecute_ValidationError_SurfacesDetail(t *testing.T) {
	// The API's validator returns structured detail (cycle,
	// unknown_dependency, etc). The action must pass it through
	// verbatim so the AI can read e.g. "cycle: task_name=a" and
	// fix the bad request.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"validation","detail":{"reason":"cycle","task_name":"a"}}`))
	}))
	defer srv.Close()

	flow := newReviseTestFlow(srv.URL)
	out, err := Execute(flow, nil, connsFromMap(map[string]string{
		"plan_id":   "plan-abc",
		"add_tasks": `[{"name":"a","depends_on":["a"]}]`,
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("cycle"))
}

func TestExecute_TerminalPlan_409(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"plan_terminal"}`))
	}))
	defer srv.Close()

	flow := newReviseTestFlow(srv.URL)
	out, err := Execute(flow, nil, connsFromMap(map[string]string{
		"plan_id":      "plan-abc",
		"remove_tasks": `["x"]`,
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("terminal"))
}

func TestExecute_NotFound_404(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	flow := newReviseTestFlow(srv.URL)
	out, err := Execute(flow, nil, connsFromMap(map[string]string{
		"plan_id":   "plan-missing",
		"add_tasks": `[{"name":"x"}]`,
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("not found"))
}

func TestExecute_MissingPlanID_FailsClean(t *testing.T) {
	RegisterTestingT(t)
	flow := newReviseTestFlow("http://unused")
	out, err := Execute(flow, nil, connsFromMap(map[string]string{
		"plan_id":   "",
		"add_tasks": `[{"name":"x"}]`,
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("plan_id is required"))
}

func TestExecute_EmptyOps_FailsCleanLocally(t *testing.T) {
	// The action catches all-empty revisions BEFORE the API call.
	// The API has its own guard (empty_revision 400) but failing
	// early saves a round-trip.
	RegisterTestingT(t)
	flow := newReviseTestFlow("http://unused")
	out, err := Execute(flow, nil, connsFromMap(map[string]string{
		"plan_id": "plan-abc",
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("at least one"))
}

func TestExecute_MalformedAddTasksJSON_FailsClean(t *testing.T) {
	RegisterTestingT(t)
	flow := newReviseTestFlow("http://unused")
	out, err := Execute(flow, nil, connsFromMap(map[string]string{
		"plan_id":   "plan-abc",
		"add_tasks": `not-json`,
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("must be a JSON array"))
}

func TestExecute_NoAPIURL_FailsFast(t *testing.T) {
	RegisterTestingT(t)
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{})
	out, err := Execute(flow, nil, connsFromMap(map[string]string{
		"plan_id":   "plan-abc",
		"add_tasks": `[{"name":"x"}]`,
	}))
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
	flow := newReviseTestFlow(srv.URL)
	flow.SetCancelContext(ctx, cancel)
	cancel()
	out, err := Execute(flow, nil, connsFromMap(map[string]string{
		"plan_id":   "plan-abc",
		"add_tasks": `[{"name":"x"}]`,
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
}
