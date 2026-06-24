package databricks_invoke_model

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
)

func newServer(t *testing.T, respBody string, gotPath *string, gotBody *map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		*gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
}

func inputs(host, endpoint, payload string) []*core.Connection {
	return []*core.Connection{
		{Name: "host", Type: core.ConnectionTypeString, Value: host},
		{Name: "token", Type: core.ConnectionTypeSecret, Value: "dapiTEST"},
		{Name: "endpoint_name", Type: core.ConnectionTypeString, Value: endpoint},
		{Name: "payload", Type: core.ConnectionTypeText, Value: payload},
	}
}

// ML model: {"predictions": [...]} — verifies path encoding, payload passthrough,
// and predictions extraction.
func TestExecute_Predictions(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	srv := newServer(t, `{"predictions":[0.92,0.08]}`, &gotPath, &gotBody)
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL, "churn-model", `{"dataframe_records":[{"tenure":12}]}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=%v, error=%v", out["success"], out["error"])
	}
	if gotPath != "/serving-endpoints/churn-model/invocations" {
		t.Errorf("path = %q, unexpected", gotPath)
	}
	recs, ok := gotBody["dataframe_records"].([]interface{})
	if !ok || len(recs) != 1 {
		t.Errorf("payload not passed through: %v", gotBody)
	}
	preds, ok := out["predictions"].([]interface{})
	if !ok || len(preds) != 2 {
		t.Fatalf("predictions = %v, want 2 values", out["predictions"])
	}
	if out["content"] != "" {
		t.Errorf("content = %v, want empty for ML response", out["content"])
	}
	t.Logf("OK — %v", out["tool_result"])
}

// Chat/LLM: OpenAI-style choices[0].message.content — verifies content extraction.
func TestExecute_ChatContent(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	srv := newServer(t, `{"choices":[{"message":{"role":"assistant","content":"Hello there"}}]}`, &gotPath, &gotBody)
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL, "llama-chat", `{"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=%v, error=%v", out["success"], out["error"])
	}
	if out["content"] != "Hello there" {
		t.Fatalf("content = %v, want 'Hello there'", out["content"])
	}
	t.Logf("OK — %v", out["tool_result"])
}

// A pre-structured payload (map/slice wired from an upstream Object output)
// must be sent as-is, without requiring the user to stringify it.
func TestExecute_StructuredPayload(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	srv := newServer(t, `{"predictions":[1]}`, &gotPath, &gotBody)
	defer srv.Close()

	in := []*core.Connection{
		{Name: "host", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "token", Type: core.ConnectionTypeSecret, Value: "dapiTEST"},
		{Name: "endpoint_name", Type: core.ConnectionTypeString, Value: "ep"},
		// payload value is a real map, as it would arrive from an upstream node.
		{Name: "payload", Type: core.ConnectionTypeText, Value: map[string]interface{}{
			"dataframe_records": []interface{}{map[string]interface{}{"x": 1}},
		}},
	}

	out, err := Execute(nil, nil, in)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=%v, error=%v", out["success"], out["error"])
	}
	if _, ok := gotBody["dataframe_records"].([]interface{}); !ok {
		t.Fatalf("structured payload not sent through: %v", gotBody)
	}
	t.Logf("OK — structured payload sent as-is")
}

// Invalid JSON payload must soft-fail before any HTTP call.
func TestExecute_BadPayload(t *testing.T) {
	out, err := Execute(nil, nil, inputs("http://unused", "ep", `not json`))
	if err != nil {
		t.Fatalf("expected soft error, got hard error: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("success=%v, want false for bad payload", out["success"])
	}
	t.Logf("OK — rejected: %v", out["error"])
}
