package azure_storage_blob_delete

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

// TestExecuteDeletesWithSnapshotsByDefault pins the header that keeps the
// delete from failing on a blob that has snapshots — n8n never sends it and
// inherits exactly that failure.
func TestExecuteDeletesWithSnapshotsByDefault(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotSnapshots string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		gotSnapshots = r.Header.Get("x-ms-delete-snapshots")
		w.Header().Set("x-ms-delete-type-permanent", "true")
		w.WriteHeader(http.StatusAccepted)
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
	if gotMethod != http.MethodDelete || gotPath != "/my-container/reports/summary%20final.pdf" || gotQuery != "" {
		t.Errorf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if gotSnapshots != "include" {
		t.Errorf("x-ms-delete-snapshots = %q, want include by default", gotSnapshots)
	}
	result := out["result"].(map[string]interface{})
	if result["deleted"] != true || result["snapshots"] != "include" {
		t.Errorf("result = %#v", result)
	}
	if out["id"] != "reports/summary final.pdf" {
		t.Errorf("id = %v", out["id"])
	}
	if !strings.Contains(out["tool_result"].(string), "Deleted reports/summary final.pdf from my-container") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecuteSnapshotsOnly — the blob survives, its snapshots do not, and the
// summary says so rather than claiming the blob was deleted.
func TestExecuteSnapshotsOnly(t *testing.T) {
	var gotSnapshots string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSnapshots = r.Header.Get("x-ms-delete-snapshots")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "summary.pdf"},
		&core.Connection{Name: "snapshots", Type: core.ConnectionTypeString, Value: "only"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if gotSnapshots != "only" {
		t.Errorf("x-ms-delete-snapshots = %q", gotSnapshots)
	}
	if !strings.Contains(out["tool_result"].(string), "Deleted the snapshots of summary.pdf") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

func TestExecuteExplicitInclude(t *testing.T) {
	var gotSnapshots string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSnapshots = r.Header.Get("x-ms-delete-snapshots")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	if _, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "summary.pdf"},
		&core.Connection{Name: "snapshots", Type: core.ConnectionTypeString, Value: "include"},
	)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotSnapshots != "include" {
		t.Errorf("x-ms-delete-snapshots = %q", gotSnapshots)
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
	if out["success"] != false || !strings.Contains(msg, "BlobNotFound: The specified blob does not exist") {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}
