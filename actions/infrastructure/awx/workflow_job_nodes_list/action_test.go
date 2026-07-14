package infrastructure_awx_workflow_job_nodes_list

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	. "github.com/onsi/gomega"
)

func awxServer(h http.HandlerFunc) *httptest.Server {
	awx.ResetAPIRootCacheForTest()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			_, _ = w.Write([]byte(`{"current_version":"/api/v2/","available_versions":{"v2":"/api/v2/"}}`))
		case "/api/v2/me/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"username":"admin"}]}`))
		default:
			h(w, r)
		}
	}))
}

func with(base string, extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: base},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}, extra...)
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// nodes covers every shape a workflow node comes in:
//   - 11 succeeded
//   - 12 FAILED
//   - 13 has no child job because do_not_run is set (the branch was not taken) —
//     ★ this must NOT be reported as a failure
//   - 14 is a workflow_approval waiting on a human — the workflow is paused, not stuck
const nodes = `{"count":4,"next":null,"results":[
	{"id":11,"do_not_run":false,"summary_fields":{"job":{"id":100,"name":"Build","type":"job","status":"successful","failed":false,"elapsed":3.1}}},
	{"id":12,"do_not_run":false,"summary_fields":{"job":{"id":101,"name":"Deploy","type":"job","status":"failed","failed":true,"elapsed":9}}},
	{"id":13,"do_not_run":true,"job":null,"summary_fields":{"unified_job_template":{"name":"Rollback"}}},
	{"id":14,"do_not_run":false,"summary_fields":{"job":{"id":102,"name":"Approve release","type":"workflow_approval","status":"pending","failed":false}}}
]}`

func TestListNodesFindsTheFailedStepAndIgnoresTheNotTakenBranch(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/workflow_jobs/99/workflow_nodes/"))
		_, _ = w.Write([]byte(nodes))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(srv.URL, str("workflow_job_id", "99")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(4))
	Expect(out["total_count"]).To(Equal(4))
	Expect(out["has_more"]).To(Equal(false))

	failed, ok := out["failed_nodes"].([]interface{})
	Expect(ok).To(BeTrue())
	Expect(failed).To(HaveLen(1), "do_not_run (a branch that was never taken) is NOT a failure")

	step := failed[0].(map[string]interface{})
	Expect(step["node_id"]).To(Equal("12"))
	Expect(step["job_id"]).To(Equal("101"))
	Expect(step["job_name"]).To(Equal("Deploy"))
	Expect(step["job_type"]).To(Equal("job"))
	Expect(step["status"]).To(Equal("failed"))

	// A pending approval is surfaced, so "why is my workflow stuck?" has an answer.
	Expect(out["tool_result"]).To(ContainSubstring("waiting for a human to approve"))
}

func TestListNodesFailedOnlyReturnsJustTheFailures(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(nodes))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(srv.URL,
		str("workflow_job_id", "99"),
		&core.Connection{Name: "failed_only", Type: core.ConnectionTypeBoolean, Value: true},
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))

	results, ok := out["results"].([]interface{})
	Expect(ok).To(BeTrue())
	Expect(results).To(HaveLen(1))
	Expect(results[0].(map[string]interface{})["node_id"]).To(Equal("12"))
	Expect(out["count"]).To(Equal(1))
	Expect(out["total_count"]).To(Equal(4), "total_count stays AWX's server-side total")
}

func TestListNodesMissingJobIDIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, with("https://awx.example.com"))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Workflow Job ID is required"))
}
