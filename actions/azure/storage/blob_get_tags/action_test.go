package azure_storage_blob_get_tags

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

// TestExecuteGetsTags — GET ?comp=tags; the XML TagSet is flattened into a
// plain object a flow can read keys off directly.
func TestExecuteGetsTags(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<Tags><TagSet>
  <Tag><Key>project</Key><Value>alpha</Value></Tag>
  <Tag><Key>status</Key><Value>final</Value></Tag>
</TagSet></Tags>`))
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
	// GET ?comp=tags is what makes this a get-tags call; the SDK escapes the blob
	// name (spaces and the virtual-directory "/") into one path segment.
	if gotMethod != http.MethodGet || gotPath != "/my-container/reports%2Fsummary%20final.pdf" || gotQuery != "comp=tags" {
		t.Errorf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	tags := out["result"].(map[string]interface{})
	if tags["project"] != "alpha" || tags["status"] != "final" {
		t.Errorf("tags = %#v", tags)
	}
	if out["id"] != "reports/summary final.pdf" {
		t.Errorf("id = %v", out["id"])
	}
	if !strings.Contains(out["tool_result"].(string), "Fetched 2 tags") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecuteNoTagsIsAnEmptyObject — an untagged blob is a success with an
// empty object, not an error.
func TestExecuteNoTagsIsAnEmptyObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><Tags><TagSet /></Tags>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "plain.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if len(out["result"].(map[string]interface{})) != 0 {
		t.Errorf("result = %#v, want an empty object", out["result"])
	}
	if !strings.Contains(out["tool_result"].(string), "Fetched 0 tags") {
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

// TestExecuteMalformedXMLIsSoftError — a body that isn't the expected envelope
// must not panic or hard-fail the node.
func TestExecuteMalformedXMLIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<Tags><TagSet><Tag><Key>oops`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// The SDK fails to unmarshal the truncated envelope; the action turns that into
	// a soft failure (an XML syntax error) rather than panicking or hard-failing.
	if out["success"] != false || !strings.Contains(out["error"].(string), "XML syntax error") {
		t.Errorf("out = %v", out)
	}
}
