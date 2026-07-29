package gitlab_get_file

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestGetFileDecodesBase64(t *testing.T) {
	RegisterTestingT(t)

	raw := "{\n  \"version\": \"1.0.0\"\n}\n"
	body := `{
	  "file_name": "app.json",
	  "file_path": "config/app.json",
	  "size": 24,
	  "encoding": "base64",
	  "content": "` + base64.StdEncoding.EncodeToString([]byte(raw)) + `",
	  "content_sha256": "abc123",
	  "ref": "main",
	  "blob_id": "blob99",
	  "last_commit_id": "commit77"
	}`

	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "glpat-test"},
		{Name: "base_url", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "project_id", Type: core.ConnectionTypeString, Value: "23"},
		{Name: "file_path", Type: core.ConnectionTypeString, Value: "config/app.json"},
		{Name: "ref", Type: core.ConnectionTypeString, Value: "main"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["content"]).To(Equal(raw))
	Expect(out["file_path"]).To(Equal("config/app.json"))
	Expect(out["file_name"]).To(Equal("app.json"))
	Expect(out["size"]).To(Equal(int64(24)))
	Expect(out["sha"]).To(Equal("abc123"))
	Expect(out["last_commit_id"]).To(Equal("commit77"))
	Expect(out["ref"]).To(Equal("main"))

	// Slashes in the path must be escaped to %2F; ref passed as a query param.
	Expect(gotURI).To(ContainSubstring("/repository/files/config%2Fapp.json"))
	Expect(gotURI).To(ContainSubstring("ref=main"))
}

func TestGetFileMissingReturnsError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 File Not Found"}`))
	}))
	defer srv.Close()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "glpat-test"},
		{Name: "base_url", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "project_id", Type: core.ConnectionTypeString, Value: "23"},
		{Name: "file_path", Type: core.ConnectionTypeString, Value: "missing.json"},
		{Name: "ref", Type: core.ConnectionTypeString, Value: "main"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("404"))
}

func TestGetFilePlainEncoding(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"file_path":"a.txt","encoding":"text","content":"hello","size":5,"ref":"main"}`))
	}))
	defer srv.Close()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "glpat-test"},
		{Name: "base_url", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "project_id", Type: core.ConnectionTypeString, Value: "23"},
		{Name: "file_path", Type: core.ConnectionTypeString, Value: "a.txt"},
		{Name: "ref", Type: core.ConnectionTypeString, Value: "main"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	// Non-base64 encoding is passed through verbatim.
	Expect(out["content"]).To(Equal("hello"))
	Expect(strings.TrimSpace(out["tool_result"].(string))).To(ContainSubstring("Read a.txt"))
}
