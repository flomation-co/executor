package azure_storage_blob_set_metadata

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

// TestExecuteSetsMetadata — PUT ?comp=metadata; each key travels as its own
// x-ms-meta-* header, and non-string scalars are coerced to strings.
func TestExecuteSetsMetadata(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		gotHeaders = r.Header.Clone()
		w.Header().Set("ETag", `"0x4"`)
		w.Header().Set("x-ms-version-id", "2026-07-16T21:00:00.0000000Z")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "reports/summary final.pdf"},
		&core.Connection{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{"reviewed":"true","revision":3}`},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if gotMethod != http.MethodPut || gotPath != "/my-container/reports/summary%20final.pdf" || gotQuery != "comp=metadata" {
		t.Errorf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if got := gotHeaders.Get("x-ms-meta-reviewed"); got != "true" {
		t.Errorf("x-ms-meta-reviewed = %q", got)
	}
	if got := gotHeaders.Get("x-ms-meta-revision"); got != "3" {
		t.Errorf("x-ms-meta-revision = %q, want a coerced scalar", got)
	}
	props := out["result"].(map[string]interface{})["properties"].(map[string]interface{})
	if props["etag"] != `"0x4"` {
		t.Errorf("properties = %#v", props)
	}
	if !strings.Contains(out["tool_result"].(string), "Set metadata on reports/summary final.pdf") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

func TestExecuteMissingMetadataIsSoftError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []*core.Connection
	}{
		{"absent", nil},
		{"empty object", []*core.Connection{{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{}`}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputs := baseInputs("http://unused.invalid",
				&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
				&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
			)
			inputs = append(inputs, tc.extra...)
			out, err := Execute(&core.Flow{}, nil, inputs)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), "metadata is required") {
				t.Errorf("out = %v", out)
			}
		})
	}
}

// TestExecuteInvalidMetadataNameIsSoftError — metadata names become header
// names and must be C# identifiers; the service's own error is opaque, so this
// is caught client-side.
func TestExecuteInvalidMetadataNameIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
		&core.Connection{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{"not-valid":"x"}`},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), `metadata name "not-valid" is invalid`) {
		t.Errorf("out = %v", out)
	}
}

func TestExecuteMalformedJSONIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
		&core.Connection{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{"broken"`},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "metadata must be valid JSON") {
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
		&core.Connection{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{"reviewed":"true"}`},
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
