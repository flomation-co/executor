package azure_storage_blob_download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// chdirWorkspace points the process cwd at a fresh temp dir — the workspace the
// downloaded bytes land in (and which flo:file: refs are relative to).
func chdirWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if real, err := filepath.EvalSymlinks(ws); err == nil {
		ws = real
	}
	old, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return ws
}

// resolveRef reads back whatever EmitMediaFile handed out. A bare Flow has no
// blob backend, so the emit falls back to a flo:file: workspace reference.
func resolveRef(t *testing.T, ws, ref string) []byte {
	t.Helper()
	rel, ok := core.ParseFileRef(ref)
	if !ok {
		t.Fatalf("file output %q is not a workspace file reference", ref)
	}
	b, err := os.ReadFile(filepath.Join(ws, rel))
	if err != nil {
		t.Fatalf("read emitted file: %v", err)
	}
	return b
}

// TestExecuteDownloadsTextBlob pins the request, the emitted media file, and
// the text-inlining rule for a small text/* blob.
func TestExecuteDownloadsTextBlob(t *testing.T) {
	ws := chdirWorkspace(t)
	var gotMethod, gotPath, gotRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Azure's SDK sends the range as x-ms-range, not the HTTP Range header.
		gotMethod, gotPath, gotRange = r.Method, r.URL.EscapedPath(), r.Header.Get("x-ms-range")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("ETag", `"0x1"`)
		w.Header().Set("x-ms-blob-type", "BlockBlob")
		w.Header().Set("x-ms-meta-source", "flomation")
		_, _ = w.Write([]byte("report body"))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "reports/2026 q3/summary.txt"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	// The SDK escapes the blob name (spaces and the virtual-directory "/") into one
	// path segment.
	if gotMethod != http.MethodGet || gotPath != "/my-container/reports%2F2026%20q3%2Fsummary.txt" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotRange != "" {
		t.Errorf("x-ms-range = %q, want unset for a whole-blob download", gotRange)
	}

	// The bytes always travel as a reference; the inline copy is a convenience.
	ref := out["file"].(string)
	if got := resolveRef(t, ws, ref); string(got) != "report body" {
		t.Errorf("emitted file = %q", got)
	}
	if out["content"] != "report body" {
		t.Errorf("content = %q, want the inlined text", out["content"])
	}
	if out["content_type"] != "text/plain; charset=utf-8" {
		t.Errorf("content_type = %v", out["content_type"])
	}
	if out["size"] != len("report body") {
		t.Errorf("size = %v", out["size"])
	}
	meta := out["result"].(map[string]interface{})["metadata"].(map[string]interface{})
	if meta["source"] != "flomation" {
		t.Errorf("metadata = %#v", meta)
	}
}

// TestExecuteRangeRequest — a byte range is passed through verbatim and the
// 206 that answers it is a success, not an error.
func TestExecuteRangeRequest(t *testing.T) {
	ws := chdirWorkspace(t)
	var gotRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The SDK carries the byte range in x-ms-range, not the HTTP Range header.
		gotRange = r.Header.Get("x-ms-range")
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Range", "bytes 0-4/11")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("repor"))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "summary.txt"},
		&core.Connection{Name: "range", Type: core.ConnectionTypeString, Value: "bytes=0-4"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("206 must succeed, error: %v", out["error"])
	}
	if gotRange != "bytes=0-4" {
		t.Errorf("Range = %q", gotRange)
	}
	if out["content"] != "repor" || out["size"] != 5 {
		t.Errorf("content = %v size = %v", out["content"], out["size"])
	}
	if got := resolveRef(t, ws, out["file"].(string)); string(got) != "repor" {
		t.Errorf("emitted file = %q", got)
	}
}

func TestExecuteMalformedRangeIsSoftError(t *testing.T) {
	chdirWorkspace(t)
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "summary.txt"},
		&core.Connection{Name: "range", Type: core.ConnectionTypeString, Value: "0-1023"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "bytes=0-1023") {
		t.Errorf("out = %v", out)
	}
}

// TestExecuteBinaryBlobIsNotInlined — only text-ish types are inlined; a binary
// blob still travels as a file reference.
func TestExecuteBinaryBlobIsNotInlined(t *testing.T) {
	ws := chdirWorkspace(t)
	payload := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "logo.png"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if out["content"] != "" {
		t.Errorf("content = %q, want empty for a binary blob", out["content"])
	}
	if out["size"] != len(payload) {
		t.Errorf("size = %v", out["size"])
	}
	if got := resolveRef(t, ws, out["file"].(string)); string(got) != string(payload) {
		t.Errorf("emitted file = %q", got)
	}
}

// TestExecuteInlinesJSONLikeTypes covers the +json/+xml suffix arm of the
// text-ish rule.
func TestExecuteInlinesJSONLikeTypes(t *testing.T) {
	chdirWorkspace(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "doc.json"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["content"] != `{"ok":true}` {
		t.Errorf("content = %v, want a +json body inlined", out["content"])
	}
}

// TestExecuteOversizeTextIsNotInlined — the inline output is capped at 1 MB;
// past that the file reference is the only carrier.
func TestExecuteOversizeTextIsNotInlined(t *testing.T) {
	ws := chdirWorkspace(t)
	big := strings.Repeat("a", inlineContentLimit+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(big))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "big.txt"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if out["content"] != "" {
		t.Errorf("content is %d bytes, want empty above the 1 MB inline cap", len(out["content"].(string)))
	}
	if out["size"] != inlineContentLimit+1 {
		t.Errorf("size = %v", out["size"])
	}
	if got := resolveRef(t, ws, out["file"].(string)); len(got) != inlineContentLimit+1 {
		t.Errorf("emitted file = %d bytes", len(got))
	}
}

func TestExecuteNotFoundIsSoftError(t *testing.T) {
	chdirWorkspace(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-error-code", "BlobNotFound")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>BlobNotFound</Code><Message>The specified blob does not exist.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "missing.txt"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	// BlobNotFound is intercepted and rendered as the friendly message (the SDK
	// classified the code, which is what HasCode branches on).
	if out["success"] != false || !strings.Contains(msg, `blob "missing.txt" was not found in "my-container"`) {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
	// A failed download must not emit a file reference.
	if _, ok := out["file"]; ok {
		t.Errorf("error output carries a file ref: %v", out["file"])
	}
}
