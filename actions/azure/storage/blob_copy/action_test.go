package azure_storage_blob_copy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// TestExecuteSameAccountCopyCompletesImmediately — a same-account source is
// turned into an authenticated URL on this account (no SAS needed), and a copy
// the service completes synchronously needs no polling at all.
func TestExecuteSameAccountCopyCompletesImmediately(t *testing.T) {
	var requests int
	var gotMethod, gotPath, gotSource string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		gotMethod, gotPath = r.Method, r.URL.EscapedPath()
		gotSource = r.Header.Get("x-ms-copy-source")
		w.Header().Set("x-ms-copy-id", "cid-1")
		w.Header().Set("x-ms-copy-status", "success")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "dest-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "backups/summary.pdf"},
		&core.Connection{Name: "source_container", Type: core.ConnectionTypeString, Value: "src-container"},
		&core.Connection{Name: "source_blob", Type: core.ConnectionTypeString, Value: "reports/summary final.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1 (a completed copy must not poll)", requests)
	}
	if gotMethod != http.MethodPut || gotPath != "/dest-container/backups/summary.pdf" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	// The source URL is built on this account, with each name segment escaped.
	wantSource := srv.URL + "/src-container/reports/summary%20final.pdf"
	if gotSource != wantSource {
		t.Errorf("x-ms-copy-source = %q, want %q", gotSource, wantSource)
	}
	if out["copy_status"] != "success" || out["copy_id"] != "cid-1" {
		t.Errorf("copy_status = %v copy_id = %v", out["copy_status"], out["copy_id"])
	}
	result := out["result"].(map[string]interface{})
	if result["sourceContainer"] != "src-container" || result["sourceBlob"] != "reports/summary final.pdf" {
		t.Errorf("result = %#v", result)
	}
	if !strings.Contains(out["tool_result"].(string), "status success") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecutePollsPendingCopyToCompletion — an async copy is polled on the
// DESTINATION with HEAD until the service reports a terminal status.
func TestExecutePollsPendingCopyToCompletion(t *testing.T) {
	var mu sync.Mutex
	var puts, heads int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			puts++
			w.Header().Set("x-ms-copy-id", "cid-2")
			w.Header().Set("x-ms-copy-status", "pending")
			w.WriteHeader(http.StatusAccepted)
		case http.MethodHead:
			heads++
			if r.URL.EscapedPath() != "/dest-container/copy.bin" {
				t.Errorf("poll path = %q, want the destination blob", r.URL.EscapedPath())
			}
			// Still running on the first poll, done on the second.
			if heads < 2 {
				w.Header().Set("x-ms-copy-status", "pending")
			} else {
				w.Header().Set("x-ms-copy-status", "success")
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected %s", r.Method)
		}
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "dest-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "copy.bin"},
		&core.Connection{Name: "source_url", Type: core.ConnectionTypeString, Value: "https://other.blob.core.windows.net/c/b.bin?sig=secret"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	mu.Lock()
	defer mu.Unlock()
	if puts != 1 || heads != 2 {
		t.Errorf("puts = %d heads = %d, want 1 PUT and 2 polls", puts, heads)
	}
	if out["copy_status"] != "success" || out["copy_id"] != "cid-2" {
		t.Errorf("copy_status = %v copy_id = %v", out["copy_status"], out["copy_id"])
	}
}

// TestExecuteRedactsTheSourceSASFromOutput — result.source is echoed into the
// run record and every downstream node, so a source_url carrying a SAS must
// lose its signature on the way out. The rest of the URL is provenance and
// stays.
func TestExecuteRedactsTheSourceSASFromOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-copy-id", "cid-5")
		w.Header().Set("x-ms-copy-status", "success")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	const sourceURL = "https://other.blob.core.windows.net/c/b.bin?sv=2023-11-03&sp=r&se=2026-07-17T10%3A00%3A00Z&sig=LIVE-SAS-SIGNATURE"
	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "dest-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "copy.bin"},
		&core.Connection{Name: "source_url", Type: core.ConnectionTypeString, Value: sourceURL},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	source := out["result"].(map[string]interface{})["source"].(string)
	if strings.Contains(source, "LIVE-SAS-SIGNATURE") {
		t.Errorf("result.source leaked the source SAS signature: %q", source)
	}
	if !strings.Contains(source, "sig=REDACTED") {
		t.Errorf("result.source = %q, want the sig slot marked redacted", source)
	}
	if !strings.Contains(source, "other.blob.core.windows.net/c/b.bin") {
		t.Errorf("result.source = %q, want the source still identifiable", source)
	}
}

// TestExecuteEchoesTheResolvedSameAccountSource — a same-account copy reports
// the URL it actually asked the service to read, which carries no SAS.
func TestExecuteEchoesTheResolvedSameAccountSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-copy-id", "cid-6")
		w.Header().Set("x-ms-copy-status", "success")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "dest-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "copy.bin"},
		&core.Connection{Name: "source_container", Type: core.ConnectionTypeString, Value: "src-container"},
		&core.Connection{Name: "source_blob", Type: core.ConnectionTypeString, Value: "b.bin"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got, want := out["result"].(map[string]interface{})["source"], srv.URL+"/src-container/b.bin"; got != want {
		t.Errorf("result.source = %v, want %q", got, want)
	}
}

// TestExecuteWaitOffReturnsPendingImmediately — with the wait turned off the
// action reports the in-flight status and the copy id to check later.
func TestExecuteWaitOffReturnsPendingImmediately(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("x-ms-copy-id", "cid-3")
		w.Header().Set("x-ms-copy-status", "pending")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "dest-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "copy.bin"},
		&core.Connection{Name: "source_url", Type: core.ConnectionTypeString, Value: "https://example.com/b.bin"},
		&core.Connection{Name: "wait_for_completion", Type: core.ConnectionTypeBoolean, Value: false},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1 (no polling when the wait is off)", requests)
	}
	if out["copy_status"] != "pending" || out["copy_id"] != "cid-3" {
		t.Errorf("copy_status = %v copy_id = %v", out["copy_status"], out["copy_id"])
	}
}

// TestExecuteFailedCopyIsSoftError — a copy that fails server-side is a soft
// failure carrying the service's description.
func TestExecuteFailedCopyIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			w.Header().Set("x-ms-copy-id", "cid-4")
			w.Header().Set("x-ms-copy-status", "pending")
			w.WriteHeader(http.StatusAccepted)
		case http.MethodHead:
			w.Header().Set("x-ms-copy-status", "failed")
			w.Header().Set("x-ms-copy-status-description", "500 InternalServerError \"Copy failed.\"")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "dest-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "copy.bin"},
		&core.Connection{Name: "source_url", Type: core.ConnectionTypeString, Value: "https://example.com/b.bin"},
	))
	if err != nil {
		t.Fatalf("a failed copy must be a soft failure, got %v", err)
	}
	msg := out["error"].(string)
	if out["success"] != false || !strings.Contains(msg, "copy to copy.bin failed") || !strings.Contains(msg, "Copy failed.") {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}

func TestExecuteSourceValidation(t *testing.T) {
	cases := []struct {
		name  string
		extra []*core.Connection
		want  string
	}{
		{
			name: "both source styles",
			extra: []*core.Connection{
				{Name: "source_url", Type: core.ConnectionTypeString, Value: "https://example.com/b.bin"},
				{Name: "source_container", Type: core.ConnectionTypeString, Value: "src"},
				{Name: "source_blob", Type: core.ConnectionTypeString, Value: "b.bin"},
			},
			want: "not both",
		},
		{
			name:  "no source at all",
			extra: nil,
			want:  "provide source_url, or both source_container and source_blob",
		},
		{
			name: "half a same-account source",
			extra: []*core.Connection{
				{Name: "source_container", Type: core.ConnectionTypeString, Value: "src"},
			},
			want: "provide source_url, or both source_container and source_blob",
		},
		{
			name: "non-http source url",
			extra: []*core.Connection{
				{Name: "source_url", Type: core.ConnectionTypeString, Value: "file:///etc/passwd"},
			},
			want: "source_url must be an http(s) URL",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := baseInputs("http://unused.invalid",
				&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "dest-container"},
				&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "copy.bin"},
			)
			inputs = append(inputs, tc.extra...)
			out, err := Execute(&core.Flow{}, nil, inputs)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), tc.want) {
				t.Errorf("out = %v, want an error containing %q", out, tc.want)
			}
		})
	}
}

func TestExecuteAPIErrorIsSoft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>ContainerNotFound</Code><Message>The specified container does not exist.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "dest-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "copy.bin"},
		&core.Connection{Name: "source_url", Type: core.ConnectionTypeString, Value: "https://example.com/b.bin"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "ContainerNotFound") {
		t.Errorf("out = %v", out)
	}
}
