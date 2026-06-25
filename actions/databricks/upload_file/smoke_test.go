package databricks_upload_file

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
)

// TestExecute_SmokeTest mocks the Files API PUT endpoint and verifies Execute
// base64-decodes content, encodes the path (including a space), and PUTs the raw
// bytes with overwrite=true.
func TestExecute_SmokeTest(t *testing.T) {
	var gotPath, gotQuery string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		gotPath = r.URL.EscapedPath()
		gotQuery = r.URL.Query().Get("overwrite")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	payload := []byte("hello,world\n1,2\n")
	b64 := base64.StdEncoding.EncodeToString(payload)

	inputs := []*core.Connection{
		{Name: "host", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "token", Type: core.ConnectionTypeSecret, Value: "dapiTEST"},
		{Name: "path", Type: core.ConnectionTypeString, Value: "/Volumes/main/default/vol/my file.csv"},
		{Name: "content", Type: core.ConnectionTypeText, Value: b64},
		{Name: "is_base64", Type: core.ConnectionTypeBoolean, Value: true},
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

	// Path must be encoded (space -> %20) under the Files API prefix.
	wantPath := "/api/2.0/fs/files/Volumes/main/default/vol/my%20file.csv"
	if gotPath != wantPath {
		t.Errorf("request path = %q, want %q", gotPath, wantPath)
	}
	if gotQuery != "true" {
		t.Errorf("overwrite query = %q, want true", gotQuery)
	}
	// Raw decoded bytes must be sent, not the base64 string.
	if string(gotBody) != string(payload) {
		t.Errorf("uploaded body = %q, want %q", gotBody, payload)
	}

	t.Logf("smoke test OK: %v", out["tool_result"])
}
