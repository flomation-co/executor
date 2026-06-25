package databricks_download_file

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
)

// TestExecute_SmokeTest mocks the Files API GET endpoint and verifies Execute
// returns the file bytes base64-encoded.
func TestExecute_SmokeTest(t *testing.T) {
	payload := []byte("col_a,col_b\n1,2\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/2.0/fs/files/Volumes/main/default/vol/out.csv" {
			t.Errorf("path = %q, unexpected", r.URL.Path)
		}
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	inputs := []*core.Connection{
		{Name: "host", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "token", Type: core.ConnectionTypeSecret, Value: "dapiTEST"},
		{Name: "path", Type: core.ConnectionTypeString, Value: "/Volumes/main/default/vol/out.csv"},
	}

	out, err := Execute(nil, nil, inputs)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, want true (error: %v)", out["success"], out["error"])
	}
	if out["size"] != len(payload) {
		t.Errorf("size = %v, want %d", out["size"], len(payload))
	}
	wantB64 := base64.StdEncoding.EncodeToString(payload)
	if out["content"] != wantB64 {
		t.Errorf("content = %v, want %v", out["content"], wantB64)
	}

	t.Logf("smoke test OK: %v", out["tool_result"])
}

// TestExecute_OversizedSoftFails verifies a too-large file is rejected with a
// soft error rather than silently truncated (or OOMing the executor).
func TestExecute_OversizedSoftFails(t *testing.T) {
	const tooBig = 51 << 20 // just over the 50 MB cap

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.CopyN(w, zeroReader{}, tooBig)
	}))
	defer srv.Close()

	inputs := []*core.Connection{
		{Name: "host", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "token", Type: core.ConnectionTypeSecret, Value: "dapiTEST"},
		{Name: "path", Type: core.ConnectionTypeString, Value: "/Volumes/main/default/vol/huge.bin"},
	}

	out, err := Execute(nil, nil, inputs)
	if err != nil {
		t.Fatalf("expected soft error, got hard error: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("success = %v, want false for oversized file", out["success"])
	}
	if out["content"] != nil {
		t.Fatalf("content should be nil on overflow, got %v", out["content"])
	}
	t.Logf("OK — rejected: %v", out["error"])
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
