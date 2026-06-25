package databricks_get_run

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
)

// TestExecute_SmokeTest mocks the Jobs runs/get endpoint and verifies Execute
// passes run_id as a query param and extracts the run state fields.
func TestExecute_SmokeTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/2.1/jobs/runs/get" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.URL.Query().Get("run_id"); got != "987654" {
			t.Errorf("run_id query = %q, want 987654", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"run_id":987654,
			"state":{"life_cycle_state":"TERMINATED","result_state":"SUCCESS","state_message":"done"},
			"run_page_url":"https://example.cloud.databricks.com/#job/42/run/12"
		}`))
	}))
	defer srv.Close()

	inputs := []*core.Connection{
		{Name: "host", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "token", Type: core.ConnectionTypeSecret, Value: "dapiTEST"},
		{Name: "run_id", Type: core.ConnectionTypeInteger, Value: int64(987654)},
	}

	out, err := Execute(nil, nil, inputs)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if out["success"] != true {
		t.Errorf("success = %v, want true (error: %v)", out["success"], out["error"])
	}
	if out["life_cycle_state"] != "TERMINATED" {
		t.Errorf("life_cycle_state = %v, want TERMINATED", out["life_cycle_state"])
	}
	if out["result_state"] != "SUCCESS" {
		t.Errorf("result_state = %v, want SUCCESS", out["result_state"])
	}
	if _, ok := out["run"].(map[string]interface{}); !ok {
		t.Errorf("run output is %T, want map", out["run"])
	}

	t.Logf("smoke test OK: %v", out["tool_result"])
}
