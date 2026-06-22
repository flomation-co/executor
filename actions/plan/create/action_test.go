package create

// Tests for the plan/create action. The action is a thin HTTP wrapper
// around the API's /api/v1/internal/agent/:id/plan endpoint, so the
// tests stand up an httptest server that mimics that endpoint and
// verify:
//
//   - happy path: 201 → tool_result + plan_id + task_count
//   - missing required inputs → clear tool_result, no API call
//   - malformed tasks_json → clear tool_result, no API call
//   - API error (4xx) → surface body verbatim so the model can
//     self-correct
//   - context missing AgentID → fail fast
//
// We deliberately don't spin up a real core.Flow context — the
// action signature takes one but only reads ctx.APIURL, ctx.AgentID,
// ctx.UserID, ctx.OrganisationID, ctx.ExecutionID and uses
// ctx.InternalClient() / flow.GoContext(). The test builds a Flow
// with those fields populated via flow.NewWithContext (or similar
// constructor exposed by core).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// newPlanTestFlow returns a Flow with an ExecutionContext that
// points at the supplied API URL. The Flow has no nodes — Execute
// only needs the context and connections.
func newPlanTestFlow(apiURL, agentID string) *core.Flow {
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{
		APIURL:         apiURL,
		AgentID:        agentID,
		UserID:         "user-andy",
		OrganisationID: "org-flomation",
		ExecutionID:    "exec-current",
	})
	return flow
}

// connsFromMap builds a slice of *core.Connection from a name→value
// map, mirroring the shape the flow engine passes to Execute.
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

func TestExecute_HappyPath_ReturnsPlanID(t *testing.T) {
	RegisterTestingT(t)

	var sawAgentInPath, sawTitle string
	var sawTaskCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expect POST /api/v1/internal/agent/<agentID>/plan
		Expect(r.Method).To(Equal(http.MethodPost))
		parts := strings.Split(r.URL.Path, "/")
		sawAgentInPath = parts[len(parts)-2] // .../agent/<id>/plan

		var body struct {
			Title          string                   `json:"title"`
			Goal           string                   `json:"goal"`
			Tasks          []map[string]interface{} `json:"tasks"`
			OwnerUserID    string                   `json:"owner_user_id"`
			OrganisationID string                   `json:"organisation_id"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
		sawTitle = body.Title
		sawTaskCount = len(body.Tasks)
		Expect(body.OwnerUserID).To(Equal("user-andy"))
		Expect(body.OrganisationID).To(Equal("org-flomation"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"plan_id":"plan-xyz","task_count":2,"status":"active"}`))
	}))
	defer srv.Close()

	flow := newPlanTestFlow(srv.URL, "agent-1")
	inputs := connsFromMap(map[string]string{
		"title": "Q3 review",
		"goal":  "Pull metrics and send for sign-off",
		"tasks_json": `[
			{"name":"pull","flow_id":"f1","flow_revision_id":"r1"},
			{"name":"send","flow_id":"f2","flow_revision_id":"r2","depends_on":["pull"]}
		]`,
	})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["plan_id"]).To(Equal("plan-xyz"))
	Expect(out["task_count"]).To(BeNumerically("==", 2))
	Expect(out["status"]).To(Equal("active"))
	Expect(out["tool_result"]).To(ContainSubstring("Plan"))
	Expect(out["tool_result"]).To(ContainSubstring("2 tasks"))

	// Confirm the wire shape the API receives.
	Expect(sawAgentInPath).To(Equal("agent-1"))
	Expect(sawTitle).To(Equal("Q3 review"))
	Expect(sawTaskCount).To(Equal(2))
}

func TestExecute_NoAgentContext_FailsFast(t *testing.T) {
	RegisterTestingT(t)
	flow := &core.Flow{}
	flow.SetContext(&core.ExecutionContext{}) // no APIURL, no AgentID
	inputs := connsFromMap(map[string]string{
		"title":      "x",
		"goal":       "y",
		"tasks_json": `[{"name":"a","flow_id":"f","flow_revision_id":"r"}]`,
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("agent context"))
}

func TestExecute_MissingTitle_FailsClean(t *testing.T) {
	RegisterTestingT(t)
	flow := newPlanTestFlow("http://unused", "agent-1")
	inputs := connsFromMap(map[string]string{
		"goal":       "y",
		"tasks_json": `[{"name":"a","flow_id":"f","flow_revision_id":"r"}]`,
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("required"))
}

func TestExecute_MalformedTasksJSON_FailsClean(t *testing.T) {
	RegisterTestingT(t)
	flow := newPlanTestFlow("http://unused", "agent-1")
	inputs := connsFromMap(map[string]string{
		"title":      "x",
		"goal":       "y",
		"tasks_json": `not-an-array`,
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("JSON array"))
}

func TestExecute_EmptyTasksArray_FailsClean(t *testing.T) {
	RegisterTestingT(t)
	flow := newPlanTestFlow("http://unused", "agent-1")
	inputs := connsFromMap(map[string]string{
		"title":      "x",
		"goal":       "y",
		"tasks_json": `[]`,
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("at least one task"))
}

func TestExecute_APIError_SurfacesBodyVerbatim(t *testing.T) {
	// The API's 400 response carries task_name + reason; the action
	// must surface that verbatim so the model can self-correct.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"validation","detail":{"reason":"duplicate_task_name","task_name":"ingest"}}`))
	}))
	defer srv.Close()

	flow := newPlanTestFlow(srv.URL, "agent-1")
	inputs := connsFromMap(map[string]string{
		"title":      "x",
		"goal":       "y",
		"tasks_json": `[{"name":"a","flow_id":"f","flow_revision_id":"r"}]`,
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("400"))
	Expect(out["tool_result"]).To(ContainSubstring("duplicate_task_name"))
	Expect(out["tool_result"]).To(ContainSubstring("ingest"))
}

func TestExecute_ContextCancellation_PropagatesViaFlow(t *testing.T) {
	// Sanity check that the request honours flow.GoContext() — a
	// cancelled flow context aborts the API call without hanging.
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hang until the request context cancels.
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	flow := newPlanTestFlow(srv.URL, "agent-1")
	flow.SetCancelContext(ctx, cancel)
	cancel()
	inputs := connsFromMap(map[string]string{
		"title":      "x",
		"goal":       "y",
		"tasks_json": `[{"name":"a","flow_id":"f","flow_revision_id":"r"}]`,
	})
	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
}

// === M1.5 commit 5: orchestrator-kind tasks_json shape ===

func TestExecute_OrchestratorKindTasks_PassesNoFlowFields(t *testing.T) {
	// Per M1.5: a task without flow_id falls back to orchestrator
	// dispatch. The wire shape is just (name, description, inputs).
	// The executor action passes the array through verbatim — the
	// API derives task_kind from field presence.
	RegisterTestingT(t)

	var sawTasks []map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Tasks []map[string]interface{} `json:"tasks"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sawTasks = body.Tasks
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"plan_id":"plan-orc","task_count":2,"status":"active"}`))
	}))
	defer srv.Close()

	flow := newPlanTestFlow(srv.URL, "agent-1")
	inputs := connsFromMap(map[string]string{
		"title": "Orchestrator-only",
		"goal":  "Both tasks dispatched via agent orchestrator",
		"tasks_json": `[
			{"name":"analyse","description":"summarise the inputs"},
			{"name":"reply","description":"send the summary back","depends_on":["analyse"]}
		]`,
	})

	out, err := Execute(flow, nil, inputs)
	Expect(err).NotTo(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["plan_id"]).To(Equal("plan-orc"))

	// API received both tasks with NO flow_id / flow_revision_id —
	// the API's deriveTaskKind sees absence and stamps the rows
	// task_kind='orchestrator'.
	Expect(sawTasks).To(HaveLen(2))
	for _, task := range sawTasks {
		Expect(task).NotTo(HaveKey("flow_id"))
		Expect(task).NotTo(HaveKey("flow_revision_id"))
	}
	Expect(sawTasks[0]["description"]).To(Equal("summarise the inputs"))
	Expect(sawTasks[1]["depends_on"]).To(Equal([]interface{}{"analyse"}))
}
