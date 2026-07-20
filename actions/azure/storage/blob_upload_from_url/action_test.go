package azure_storage_blob_upload_from_url

import (
	"io"
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

// TestExecuteCopiesFromURL pins the sync copy-source request, including the
// Content-Length: 0 the operation requires on the wire (it signs as an EMPTY
// slot, which is why n8n dropped the header altogether).
func TestExecuteCopiesFromURL(t *testing.T) {
	var (
		gotMethod, gotPath, gotQuery string
		gotSource, gotBlobType       string
		gotContentLength             string
		gotDeclaredLength            int64
		gotBody                      []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		gotSource = r.Header.Get("x-ms-copy-source")
		gotBlobType = r.Header.Get("x-ms-blob-type")
		gotContentLength = r.Header.Get("Content-Length")
		gotDeclaredLength = r.ContentLength
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("ETag", `"0x2"`)
		w.Header().Set("x-ms-content-crc64", "abc123")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "downloads/report final.pdf"},
		&core.Connection{Name: "source_url", Type: core.ConnectionTypeString, Value: "https://example.com/file.pdf?token=abc"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	// The SDK owns the request path and percent-encodes the whole blob name
	// (slashes included), so it is only checked loosely; the per-segment
	// escaping the action performs is asserted on out["id"]/the output below.
	if gotMethod != http.MethodPut || gotQuery != "" || !strings.HasPrefix(gotPath, "/my-container/") {
		t.Errorf("request = %s %s?%s, want a PUT under /my-container/", gotMethod, gotPath, gotQuery)
	}
	if gotSource != "https://example.com/file.pdf?token=abc" {
		t.Errorf("x-ms-copy-source = %q", gotSource)
	}
	if gotBlobType != "BlockBlob" {
		t.Errorf("x-ms-blob-type = %q", gotBlobType)
	}
	if gotContentLength != "0" || gotDeclaredLength != 0 || len(gotBody) != 0 {
		t.Errorf("Content-Length header = %q (declared %d), body = %q — the operation needs an explicit zero length with an empty body",
			gotContentLength, gotDeclaredLength, gotBody)
	}
	if out["id"] != "downloads/report final.pdf" {
		t.Errorf("id = %v", out["id"])
	}
	// The action shapes the result's properties from the SDK's typed put-from-URL
	// response: the ETag (and Last-Modified when present). The pre-SDK
	// contentCrc64 slot is no longer part of the output surface.
	props := out["result"].(map[string]interface{})["properties"].(map[string]interface{})
	if props["etag"] != `"0x2"` {
		t.Errorf("properties = %#v, want the ETag carried through", props)
	}
}

func TestExecuteRejectsNonHTTPSource(t *testing.T) {
	for _, bad := range []string{"ftp://example.com/f.pdf", "example.com/f.pdf", "file:///etc/passwd"} {
		out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
			&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
			&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "f.pdf"},
			&core.Connection{Name: "source_url", Type: core.ConnectionTypeString, Value: bad},
		))
		if err != nil {
			t.Fatalf("Execute(%q): %v", bad, err)
		}
		if out["success"] != false || !strings.Contains(out["error"].(string), "source_url must be an http(s) URL") {
			t.Errorf("source_url %q: out = %v", bad, out)
		}
	}
}

// TestExecuteUnfetchableSourceIsFriendlySoftError — the service's opaque
// CannotVerifyCopySource is re-worded into something an operator can act on.
func TestExecuteUnfetchableSourceIsFriendlySoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>CannotVerifyCopySource</Code><Message>Could not verify the copy source within the specified time.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "f.pdf"},
		&core.Connection{Name: "source_url", Type: core.ConnectionTypeString, Value: "https://example.com/f.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	if out["success"] != false || !strings.Contains(msg, "publicly reachable or carries a valid SAS") {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}

func TestExecuteAPIErrorIsSoft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>ContainerNotFound</Code><Message>The specified container does not exist.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "f.pdf"},
		&core.Connection{Name: "source_url", Type: core.ConnectionTypeString, Value: "https://example.com/f.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "ContainerNotFound") {
		t.Errorf("out = %v", out)
	}
}
