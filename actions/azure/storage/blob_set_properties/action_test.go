package azure_storage_blob_set_properties

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

// TestExecuteSetsAllProperties — PUT ?comp=properties; every property maps to
// its x-ms-blob-* header (NOT the bare Content-* header, which would describe
// the request instead of the blob).
func TestExecuteSetsAllProperties(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		gotHeaders = r.Header.Clone()
		w.Header().Set("ETag", `"0x5"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "reports/summary final.pdf"},
		&core.Connection{Name: "content_type", Type: core.ConnectionTypeString, Value: "application/pdf"},
		&core.Connection{Name: "cache_control", Type: core.ConnectionTypeString, Value: "max-age=3600"},
		&core.Connection{Name: "content_disposition", Type: core.ConnectionTypeString, Value: `attachment; filename="summary.pdf"`},
		&core.Connection{Name: "content_encoding", Type: core.ConnectionTypeString, Value: "gzip"},
		&core.Connection{Name: "content_language", Type: core.ConnectionTypeString, Value: "en-GB"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if gotMethod != http.MethodPut || gotPath != "/my-container/reports%2Fsummary%20final.pdf" || gotQuery != "comp=properties" {
		t.Errorf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	for header, want := range map[string]string{
		"x-ms-blob-content-type":        "application/pdf",
		"x-ms-blob-cache-control":       "max-age=3600",
		"x-ms-blob-content-disposition": `attachment; filename="summary.pdf"`,
		"x-ms-blob-content-encoding":    "gzip",
		"x-ms-blob-content-language":    "en-GB",
	} {
		if got := gotHeaders.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if !strings.Contains(out["tool_result"].(string), "Set 5 properties on reports/summary final.pdf") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecuteSendsOnlySuppliedProperties — the action does not invent headers
// for the properties the operator left blank.
func TestExecuteSendsOnlySuppliedProperties(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
		&core.Connection{Name: "cache_control", Type: core.ConnectionTypeString, Value: "no-cache"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if got := gotHeaders.Get("x-ms-blob-cache-control"); got != "no-cache" {
		t.Errorf("x-ms-blob-cache-control = %q", got)
	}
	for _, unset := range []string{
		"x-ms-blob-content-type", "x-ms-blob-content-disposition",
		"x-ms-blob-content-encoding", "x-ms-blob-content-language",
	} {
		if got := gotHeaders.Get(unset); got != "" {
			t.Errorf("%s = %q, want unset", unset, got)
		}
	}
	if !strings.Contains(out["tool_result"].(string), "Set 1 properties") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecuteNoPropertiesIsSoftError — an all-blank call would CLEAR every
// property on the blob, so it is refused rather than sent.
func TestExecuteNoPropertiesIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "set at least one property") {
		t.Errorf("out = %v", out)
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
		&core.Connection{Name: "content_type", Type: core.ConnectionTypeString, Value: "application/pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	if out["success"] != false || !strings.Contains(msg, "BlobNotFound") {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}
