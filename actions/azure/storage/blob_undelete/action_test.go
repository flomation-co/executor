package azure_storage_blob_undelete

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

// TestExecuteUndeletesBlob — PUT ?comp=undelete, empty body.
func TestExecuteUndeletesBlob(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotContentLength string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		gotContentLength = r.Header.Get("Content-Length")
		w.WriteHeader(http.StatusOK)
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
	if gotMethod != http.MethodPut || gotPath != "/my-container/reports%2Fsummary%20final.pdf" || gotQuery != "comp=undelete" {
		t.Errorf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if gotContentLength != "0" {
		t.Errorf("Content-Length = %q, want 0", gotContentLength)
	}
	if out["result"].(map[string]interface{})["undeleted"] != true {
		t.Errorf("result = %#v", out["result"])
	}
	if out["id"] != "reports/summary final.pdf" {
		t.Errorf("id = %v", out["id"])
	}
	if !strings.Contains(out["tool_result"].(string), "Restored reports/summary final.pdf in my-container") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecuteSoftDeleteDisabledIsSoftError — the account-level "soft delete is
// off" refusal must reach the operator intact.
func TestExecuteSoftDeleteDisabledIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>BlobSoftDeleted</Code><Message>The blob is soft deleted but the account does not have soft delete enabled.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "gone.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	if out["success"] != false || !strings.Contains(msg, "BlobSoftDeleted") {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}
