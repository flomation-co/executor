package azure_storage_blob_set_tier

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

const testKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

func baseInputs(endpoint string, extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
	}
	return append(inputs, extra...)
}

// TestExecuteSetsTier — PUT ?comp=tier with the tier header; a 200 means the
// change took effect immediately.
func TestExecuteSetsTier(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotTier, gotPriority string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		gotTier = r.Header.Get("x-ms-access-tier")
		gotPriority = r.Header.Get("x-ms-rehydrate-priority")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "reports/summary final.pdf"},
		&core.Connection{Name: "access_tier", Type: core.ConnectionTypeString, Value: "Cool"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if gotMethod != http.MethodPut || gotPath != "/my-container/reports/summary%20final.pdf" || gotQuery != "comp=tier" {
		t.Errorf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if gotTier != "Cool" {
		t.Errorf("x-ms-access-tier = %q", gotTier)
	}
	if gotPriority != "" {
		t.Errorf("x-ms-rehydrate-priority = %q, want unset when no priority is chosen", gotPriority)
	}
	result := out["result"].(map[string]interface{})
	if result["accessTier"] != "Cool" || result["pending"] != false {
		t.Errorf("result = %#v", result)
	}
	if !strings.Contains(out["tool_result"].(string), "Set tier of reports/summary final.pdf to Cool") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecuteRehydrationIsAccepted — leaving Archive is asynchronous: the 202
// must be reported as a background rehydration, not as a completed change.
func TestExecuteRehydrationIsAccepted(t *testing.T) {
	var gotPriority string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPriority = r.Header.Get("x-ms-rehydrate-priority")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "cold.pdf"},
		&core.Connection{Name: "access_tier", Type: core.ConnectionTypeString, Value: "Hot"},
		&core.Connection{Name: "rehydrate_priority", Type: core.ConnectionTypeString, Value: "High"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if gotPriority != "High" {
		t.Errorf("x-ms-rehydrate-priority = %q", gotPriority)
	}
	if out["result"].(map[string]interface{})["pending"] != true {
		t.Errorf("result = %#v, want pending on a 202", out["result"])
	}
	if !strings.Contains(out["tool_result"].(string), "Rehydration of cold.pdf to Hot accepted") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

func TestExecuteMissingTierIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "access_tier is required") {
		t.Errorf("out = %v", out)
	}
}

func TestExecuteArchivedBlobErrorIsSoft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>BlobArchived</Code><Message>This operation is not permitted on an archived blob.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
		&core.Connection{Name: "access_tier", Type: core.ConnectionTypeString, Value: "Archive"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	if out["success"] != false || !strings.Contains(msg, "BlobArchived: This operation is not permitted on an archived blob") {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}
