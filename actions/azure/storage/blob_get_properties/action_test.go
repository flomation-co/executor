package azure_storage_blob_get_properties

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

// TestExecuteHeadsTheBlob — properties come from response headers on a HEAD;
// nothing is downloaded (n8n GETs the whole body for the same information).
func TestExecuteHeadsTheBlob(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", "2048")
		w.Header().Set("ETag", `"0x8DA"`)
		w.Header().Set("Last-Modified", "Mon, 27 Jul 2026 12:00:00 GMT")
		w.Header().Set("x-ms-blob-type", "BlockBlob")
		w.Header().Set("x-ms-access-tier", "Hot")
		w.Header().Set("x-ms-lease-status", "unlocked")
		w.Header().Set("x-ms-server-encrypted", "true")
		w.Header().Set("x-ms-meta-project", "alpha")
		// Transport noise that must be filtered out of the properties object.
		w.Header().Set("x-ms-request-id", "req-1")
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
	if gotMethod != http.MethodHead || gotPath != "/my-container/reports/summary%20final.pdf" || gotQuery != "" {
		t.Errorf("request = %s %s?%s, want HEAD on the escaped blob path", gotMethod, gotPath, gotQuery)
	}

	result := out["result"].(map[string]interface{})
	props := result["properties"].(map[string]interface{})
	if props["blobType"] != "BlockBlob" || props["accessTier"] != "Hot" || props["leaseStatus"] != "unlocked" {
		t.Errorf("properties = %#v", props)
	}
	// Booleans and integers are coerced so a flow can branch on them directly.
	if props["serverEncrypted"] != true {
		t.Errorf("serverEncrypted = %#v, want a real boolean", props["serverEncrypted"])
	}
	if props["contentLength"] != int64(2048) {
		t.Errorf("contentLength = %#v, want an integer", props["contentLength"])
	}
	if props["etag"] != `"0x8DA"` {
		t.Errorf("etag = %v", props["etag"])
	}
	if _, noise := props["requestId"]; noise {
		t.Errorf("transport headers leaked into properties: %#v", props)
	}
	meta := result["metadata"].(map[string]interface{})
	if meta["project"] != "alpha" {
		t.Errorf("metadata = %#v", meta)
	}
	if out["id"] != "reports/summary final.pdf" {
		t.Errorf("id = %v", out["id"])
	}
}

// TestExecuteNotFoundIsSoftError — a HEAD carries no XML body, so the error
// code has to come off the x-ms-error-code header.
func TestExecuteNotFoundIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-error-code", "BlobNotFound")
		w.WriteHeader(http.StatusNotFound)
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
	if out["success"] != false || !strings.Contains(msg, "BlobNotFound") || !strings.Contains(msg, "404") {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}

// TestExecutePermissionErrorIsSoftError — an RBAC refusal on a HEAD arrives as
// a status plus x-ms-error-code and nothing else (HTTP forbids a body on a HEAD
// response, so the XML <Error> envelope never reaches us here).
func TestExecutePermissionErrorIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-error-code", "AuthorizationPermissionMismatch")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>AuthorizationPermissionMismatch</Code><Message>This request is not authorized.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "AuthorizationPermissionMismatch") {
		t.Errorf("out = %v", out)
	}
}

// TestExecuteTransportFailureIsSoftAndRedacted — a dead endpoint is an operator
// problem, not a programmer one, and the message must not carry credentials.
func TestExecuteTransportFailureIsSoftAndRedacted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	dead := srv.URL
	srv.Close() // nothing is listening now

	out, err := Execute(&core.Flow{}, nil, baseInputs(dead,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
	))
	if err != nil {
		t.Fatalf("transport failures must be soft, got %v", err)
	}
	msg := out["error"].(string)
	if out["success"] != false || !strings.Contains(msg, "Azure Storage request failed") {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}

func TestExecuteMissingBlobNameIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "blob_name is required") {
		t.Errorf("out = %v", out)
	}
}
