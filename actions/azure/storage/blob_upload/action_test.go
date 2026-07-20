package azure_storage_blob_upload

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// chdirWorkspace points the process cwd at a fresh temp dir, which is the
// workspace flo:file: references resolve against.
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

// TestExecuteUploadsInlineText pins the whole request shape: the escaped path
// (the blob name carries a space and a virtual directory), every pinned header,
// and the body — plus the outputs the action adds on top of the baseline.
func TestExecuteUploadsInlineText(t *testing.T) {
	var (
		gotMethod, gotPath, gotQuery string
		gotHeaders                   http.Header
		gotBody                      []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("ETag", `"0x8DABCDEF"`)
		w.Header().Set("x-ms-request-server-encrypted", "true")
		w.Header().Set("Last-Modified", "Mon, 27 Jul 2026 12:00:00 GMT")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "reports/2026 q3/summary final.txt"},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: "hello"},
		&core.Connection{Name: "content_type", Type: core.ConnectionTypeString, Value: "text/plain"},
		&core.Connection{Name: "access_tier", Type: core.ConnectionTypeString, Value: "Cool"},
		&core.Connection{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{"source":"flomation"}`},
		&core.Connection{Name: "tags", Type: core.ConnectionTypeObject, Value: `{"status":"final","project":"alpha"}`},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error: %v)", out["success"], out["error"])
	}

	// The action builds the OUTPUT url by escaping each blob-name segment
	// individually, so the virtual-directory slashes survive and only the space
	// is escaped (n8n interpolates raw and breaks) — that is pinned on out["url"]
	// below. The SDK owns the REQUEST path and percent-encodes the whole blob
	// name (slashes included), so the request itself is only checked loosely.
	wantPath := "/my-container/reports/2026%20q3/summary%20final.txt"
	if gotMethod != http.MethodPut || gotQuery != "" || !strings.HasPrefix(gotPath, "/my-container/") {
		t.Errorf("request = %s %s?%s, want a PUT under /my-container/", gotMethod, gotPath, gotQuery)
	}
	if got := gotHeaders.Get("x-ms-blob-type"); got != "BlockBlob" {
		t.Errorf("x-ms-blob-type = %q, want BlockBlob", got)
	}
	if got := gotHeaders.Get("x-ms-access-tier"); got != "Cool" {
		t.Errorf("x-ms-access-tier = %q", got)
	}
	if got := gotHeaders.Get("x-ms-meta-source"); got != "flomation" {
		t.Errorf("x-ms-meta-source = %q", got)
	}
	// x-ms-tags is a querystring-encoded pair list. The SDK does not sort the
	// pairs (Go map iteration order), so parse the header and assert both pairs
	// are present rather than pinning an order.
	tagVals, err := url.ParseQuery(gotHeaders.Get("x-ms-tags"))
	if err != nil {
		t.Fatalf("x-ms-tags %q: %v", gotHeaders.Get("x-ms-tags"), err)
	}
	if tagVals.Get("status") != "final" || tagVals.Get("project") != "alpha" {
		t.Errorf("x-ms-tags = %q, want status=final and project=alpha", gotHeaders.Get("x-ms-tags"))
	}
	// The blob's content type travels as x-ms-blob-content-type; the request's
	// own Content-Type is the octet-stream body type the SDK sends.
	if got := gotHeaders.Get("x-ms-blob-content-type"); got != "text/plain" {
		t.Errorf("x-ms-blob-content-type = %q, want text/plain", got)
	}
	if got := gotHeaders.Get("If-None-Match"); got != "" {
		t.Errorf("If-None-Match = %q, want unset when overwrite is on", got)
	}
	if gotHeaders.Get("x-ms-version") == "" || gotHeaders.Get("x-ms-date") == "" {
		t.Errorf("x-ms-version/x-ms-date must be pinned on every request: %v", gotHeaders)
	}
	if !strings.HasPrefix(gotHeaders.Get("Authorization"), "SharedKey devstoreaccount1:") {
		t.Errorf("Authorization = %q", gotHeaders.Get("Authorization"))
	}
	if string(gotBody) != "hello" {
		t.Errorf("body = %q", gotBody)
	}

	if out["id"] != "reports/2026 q3/summary final.txt" {
		t.Errorf("id = %v", out["id"])
	}
	if out["etag"] != `"0x8DABCDEF"` {
		t.Errorf("etag = %v", out["etag"])
	}
	if out["url"] != srv.URL+wantPath {
		t.Errorf("url = %v, want %v", out["url"], srv.URL+wantPath)
	}
	// WriteResult shapes the result's properties from the SDK's typed write
	// response: the ETag and the Last-Modified. (The pre-SDK requestServerEncrypted
	// slot is no longer part of the output surface.)
	props := out["result"].(map[string]interface{})["properties"].(map[string]interface{})
	if props["etag"] != `"0x8DABCDEF"` {
		t.Errorf("properties.etag = %v, want the write's ETag", props["etag"])
	}
	if _, ok := props["lastModified"].(string); !ok {
		t.Errorf("properties = %#v, want the Last-Modified carried through", props)
	}
	if !strings.Contains(out["tool_result"].(string), "5 bytes") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecuteResolvesBase64Content covers the base64 arm of ResolveToBytes:
// the wire must carry the DECODED bytes.
func TestExecuteResolvesBase64Content(t *testing.T) {
	original := "this payload is long enough to be recognised as base64"
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "payload.bin"},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: base64.StdEncoding.EncodeToString([]byte(original))},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if string(gotBody) != original {
		t.Errorf("body = %q, want the decoded bytes %q", gotBody, original)
	}
}

// TestExecuteResolvesFileRefContent covers the flo:file: arm: bytes come off
// the workspace and the MIME type is derived from the resolved file when no
// content_type is given.
func TestExecuteResolvesFileRefContent(t *testing.T) {
	ws := chdirWorkspace(t)
	var gotBody []byte
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotContentType = r.Header.Get("x-ms-blob-content-type")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	src := filepath.Join(ws, "note.txt")
	if err := os.WriteFile(src, []byte("from the workspace"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "note.txt"},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: "flo:file:note.txt"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if string(gotBody) != "from the workspace" {
		t.Errorf("body = %q", gotBody)
	}
	if !strings.HasPrefix(gotContentType, "text/plain") {
		t.Errorf("x-ms-blob-content-type = %q, want the type resolved from the file", gotContentType)
	}
}

// TestExecuteUnresolvableFileRefIsSoftError — a crafted ref that escapes the
// workspace must not be a hard error.
func TestExecuteUnresolvableFileRefIsSoftError(t *testing.T) {
	chdirWorkspace(t)
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "x.bin"},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: "flo:file:../../etc/passwd"},
	))
	if err != nil {
		t.Fatalf("resolve failures must be soft, got %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "resolve content") {
		t.Errorf("out = %v", out)
	}
}

// TestExecuteOverwriteOffSendsIfNoneMatch pins the conditional-create contract
// and the friendly 409 mapping.
func TestExecuteOverwriteOffSendsIfNoneMatch(t *testing.T) {
	var gotIfNoneMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfNoneMatch = r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>BlobAlreadyExists</Code><Message>The specified blob already exists.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "report.txt"},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: "hi"},
		&core.Connection{Name: "overwrite", Type: core.ConnectionTypeBoolean, Value: false},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotIfNoneMatch != "*" {
		t.Errorf("If-None-Match = %q, want * when overwrite is off", gotIfNoneMatch)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "Overwrite is off") {
		t.Errorf("out = %v", out)
	}
}

// TestExecuteConditionNotMetIsMappedToo — some service versions answer the
// If-None-Match precondition with 412 ConditionNotMet rather than 409
// BlobAlreadyExists; both must read the same way to an operator.
func TestExecuteConditionNotMetIsMappedToo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>ConditionNotMet</Code><Message>The condition specified using HTTP conditional header(s) is not met.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "report.txt"},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: "hi"},
		&core.Connection{Name: "overwrite", Type: core.ConnectionTypeBoolean, Value: false},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), `blob "report.txt" already exists in "my-container" and Overwrite is off`) {
		t.Errorf("out = %v", out)
	}
}

// TestExecuteAPIErrorIsSoftAndRedacted checks the service's XML error envelope
// surfaces as a soft error carrying the code and message, and that no credential
// material rides along.
func TestExecuteAPIErrorIsSoftAndRedacted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>AuthenticationFailed</Code><Message>Server failed to authenticate the request.
RequestId:00000000-0000-0000-0000-000000000000
Time:2026-07-16T12:00:00.0000000Z</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "report.txt"},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: "hi"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	// The SDK surfaces a service error as a verbose block: the ERROR CODE line
	// plus the raw <Error><Code>…</Code><Message>…</Message></Error> XML. Assert
	// the service's code and message both survive into the soft error.
	if out["success"] != false ||
		!strings.Contains(msg, "AuthenticationFailed") ||
		!strings.Contains(msg, "Server failed to authenticate the request") {
		t.Errorf("out = %v, want the service's code and message", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
	if out["tool_result"] != msg {
		t.Errorf("tool_result must mirror the error: %v", out["tool_result"])
	}
}

func TestExecuteMissingContentIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "report.txt"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "content is required") {
		t.Errorf("out = %v", out)
	}
}

func TestExecuteRejectsTooManyTags(t *testing.T) {
	tags := "{"
	for i := 0; i < 11; i++ {
		if i > 0 {
			tags += ","
		}
		tags += `"k` + string(rune('a'+i)) + `":"v"`
	}
	tags += "}"

	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "report.txt"},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: "hi"},
		&core.Connection{Name: "tags", Type: core.ConnectionTypeObject, Value: tags},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "at most 10 index tags") {
		t.Errorf("out = %v", out)
	}
}
