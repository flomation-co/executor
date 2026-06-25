package databricks_run_job

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
)

// TestExecute_SmokeTest mocks the Jobs run-now endpoint and verifies Execute
// sends the job_id + job_parameters and surfaces the returned run_id.
func TestExecute_SmokeTest(t *testing.T) {
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/2.1/jobs/run-now" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer dapiTEST" {
			t.Errorf("Authorization = %q, want Bearer dapiTEST", got)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run_id":987654,"number_in_job":12}`))
	}))
	defer srv.Close()

	inputs := []*core.Connection{
		{Name: "host", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "token", Type: core.ConnectionTypeSecret, Value: "dapiTEST"},
		{Name: "job_id", Type: core.ConnectionTypeInteger, Value: int64(42)},
		{Name: "parameters", Type: core.ConnectionTypeKeyValueArray, Value: `[{"key":"env","value":"prod"}]`},
	}

	out, err := Execute(nil, nil, inputs)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if out["success"] != true {
		t.Errorf("success = %v, want true (error: %v)", out["success"], out["error"])
	}
	if out["run_id"] != int64(987654) {
		t.Errorf("run_id = %v, want 987654", out["run_id"])
	}
	if out["number_in_job"] != int64(12) {
		t.Errorf("number_in_job = %v, want 12", out["number_in_job"])
	}

	// job_id must be sent as a JSON number (42), and parameters mapped to job_parameters.
	if gotBody["job_id"] != float64(42) {
		t.Errorf("sent job_id = %v, want 42", gotBody["job_id"])
	}
	params, ok := gotBody["job_parameters"].(map[string]interface{})
	if !ok || params["env"] != "prod" {
		t.Errorf("sent job_parameters = %v, want {env: prod}", gotBody["job_parameters"])
	}

	t.Logf("smoke test OK: %v", out["tool_result"])
}
