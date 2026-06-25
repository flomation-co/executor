package databricks_list_runs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
)

// TestExecute_LimitClamp verifies a limit above the API maximum (25) is clamped
// rather than passed through verbatim, and that job_id is forwarded.
func TestExecute_LimitClamp(t *testing.T) {
	var gotLimit, gotJob string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/2.1/jobs/runs/list" {
			t.Errorf("path = %q, unexpected", r.URL.Path)
		}
		gotLimit = r.URL.Query().Get("limit")
		gotJob = r.URL.Query().Get("job_id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"runs":[{"run_id":1}],"has_more":false}`))
	}))
	defer srv.Close()

	inputs := []*core.Connection{
		{Name: "host", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "token", Type: core.ConnectionTypeSecret, Value: "dapiTEST"},
		{Name: "job_id", Type: core.ConnectionTypeInteger, Value: int64(7)},
		{Name: "limit", Type: core.ConnectionTypeInteger, Value: int64(1000)},
	}

	out, err := Execute(nil, nil, inputs)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=%v, error=%v", out["success"], out["error"])
	}
	if gotLimit != "25" {
		t.Errorf("limit sent = %q, want 25 (clamped)", gotLimit)
	}
	if gotJob != "7" {
		t.Errorf("job_id sent = %q, want 7", gotJob)
	}
	t.Logf("OK — limit clamped to %s", gotLimit)
}
