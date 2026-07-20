package azure_storage_blob_snapshot

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

// TestExecuteCreatesSnapshot — PUT ?comp=snapshot with an empty body; the
// snapshot id (a timestamp) comes back in x-ms-snapshot.
func TestExecuteCreatesSnapshot(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotContentLength string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		gotContentLength = r.Header.Get("Content-Length")
		w.Header().Set("x-ms-snapshot", "2026-07-16T21:05:12.1234567Z")
		w.Header().Set("ETag", `"0x3"`)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "reports/summary final.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	// PUT ?comp=snapshot is what makes this a snapshot call; the SDK escapes the
	// blob name (spaces and the virtual-directory "/") into one path segment.
	if gotMethod != http.MethodPut || gotPath != "/my-container/reports%2Fsummary%20final.pdf" || gotQuery != "comp=snapshot" {
		t.Errorf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	// A bodyless PUT must still declare a zero length — the service requires it.
	if gotContentLength != "0" {
		t.Errorf("Content-Length = %q, want 0", gotContentLength)
	}
	if out["snapshot"] != "2026-07-16T21:05:12.1234567Z" {
		t.Errorf("snapshot = %v", out["snapshot"])
	}
	if out["id"] != "reports/summary final.pdf" {
		t.Errorf("id = %v", out["id"])
	}
	if !strings.Contains(out["tool_result"].(string), "2026-07-16T21:05:12.1234567Z") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

func TestExecuteNotFoundIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>BlobNotFound</Code><Message>The specified blob does not exist.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "missing.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	// BlobNotFound is intercepted and rendered as the friendly message (the SDK
	// classified the code, which is what HasCode branches on).
	if out["success"] != false || !strings.Contains(msg, `blob "missing.pdf" was not found in container "my-container"`) {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}
