package infrastructure_awx_inventory_source_sync

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	. "github.com/onsi/gomega"
)

// awxServer answers the API-root discovery probe exactly as a real upstream AWX
// 24.6.1 does, then delegates to h.
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

func inputs(url string, extra ...*core.Connection) []*core.Connection {
	base := []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: url},
		{Name: "auth_method", Type: core.ConnectionTypeString, Value: "token"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}
	return append(base, extra...)
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func boolean(name string, value bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: value}
}

func TestSyncStartsTheUpdateAndReturnsItsID(t *testing.T) {
	RegisterTestingT(t)

	var posted bool
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/inventory_sources/5/update/":
			_, _ = w.Write([]byte(`{"can_update":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/inventory_sources/5/update/":
			posted = true
			// The real 202: the id is at BOTH .inventory_update and .id.
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"inventory_update":55,"id":55,"type":"inventory_update","status":"pending"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL, str("inventory_source_id", "5")))

	Expect(err).To(BeNil())
	Expect(posted).To(BeTrue())
	Expect(out["success"]).To(Equal(true))
	Expect(out["inventory_update_id"]).To(Equal("55"))
	Expect(out["status"]).To(Equal("pending"))
	Expect(out["timed_out"]).To(Equal(false))
	Expect(out["tool_result"]).To(ContainSubstring("Started inventory sync 55"))
}

// ★ THE TRAP. AWX answers a POST to update/ with 405 Method Not Allowed — not 400
// — when can_update is false (a manual source, or an scm source with no source
// project). The node pre-checks and explains, and NEVER posts.
func TestSyncRefusesWhenTheSourceCannotBeUpdated(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			t.Error("posted the sync despite can_update=false")
		}
		if r.URL.Path == "/api/v2/inventory_sources/5/update/" {
			_, _ = w.Write([]byte(`{"can_update":false}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL, str("inventory_source_id", "5")))

	Expect(err).To(BeNil()) // SOFT failure — a 405 must not abort the flow
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("not syncable"))
	Expect(out["error"]).To(ContainSubstring("Source Project"))
	Expect(out["error"]).ToNot(ContainSubstring("405"))
}

func TestSyncWaitsAndReportsAFailedImport(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/inventory_sources/5/update/":
			_, _ = w.Write([]byte(`{"can_update":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/inventory_sources/5/update/":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"inventory_update":55,"id":55,"type":"inventory_update","status":"pending"}`))
		// The wait polls the LIST endpoint, then takes one detail GET. Note the
		// path: an inventory sync is polled on inventory_updates/, NOT jobs/.
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/inventory_updates/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":55,"status":"failed","failed":true,"finished":"2026-07-14T10:00:05Z"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/inventory_updates/55/":
			_, _ = w.Write([]byte(`{"id":55,"status":"failed","failed":true,"finished":"2026-07-14T10:00:05Z","job_explanation":"","event_processing_finished":true}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("inventory_source_id", "5"),
		boolean("wait_for_completion", true),
	))

	Expect(err).To(BeNil()) // still SOFT
	Expect(out["success"]).To(Equal(false))
	Expect(out["status"]).To(Equal("failed"))
	Expect(out["failed"]).To(Equal(true))
	Expect(out["finished"]).To(Equal(true))
	Expect(out["timed_out"]).To(Equal(false))
	Expect(out["inventory_update_id"]).To(Equal("55"))
	Expect(out["error"]).To(ContainSubstring("ended failed"))
}

func TestSyncIgnoreJobFailureSucceedsAnyway(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/inventory_sources/5/update/":
			_, _ = w.Write([]byte(`{"can_update":true}`))
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"inventory_update":55,"id":55,"type":"inventory_update"}`))
		case r.URL.Path == "/api/v2/inventory_updates/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":55,"status":"failed","failed":true,"finished":"2026-07-14T10:00:05Z"}]}`))
		default:
			_, _ = w.Write([]byte(`{"id":55,"status":"failed","failed":true,"finished":"2026-07-14T10:00:05Z"}`))
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("inventory_source_id", "5"),
		boolean("wait_for_completion", true),
		boolean("ignore_job_failure", true),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["failed"]).To(Equal(true))
}

func TestSyncMissingSourceIDIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, inputs("https://awx.example.com"))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Inventory Source is required"))
}

// A missing credential is the ONE hard error: it aborts the flow.
func TestSyncMissingTokenIsAHardError(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, []*core.Connection{
		str("awx_url", "https://awx.example.com"),
		str("inventory_source_id", "5"),
	})

	Expect(out).To(BeNil())
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("API Token is required"))
}
