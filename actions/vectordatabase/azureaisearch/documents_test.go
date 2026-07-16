// Tests for the document resource group: document_upload (@search.action
// wrapping, default mergeOrUpload, partial-failure soft-fail listing failed
// keys), document_delete (delete actions built from comma-separated keys),
// document_get (OData ('key') lookup path + $select) and document_count (the
// text/plain endpoint, BOM included). Input constructors live in
// indexes_test.go.
package azureaisearch_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	document_count "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/document_count"
	document_delete "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/document_delete"
	document_get "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/document_get"
	document_upload "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/document_upload"
)

func TestDocumentUpload(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"value":[{"key":"1","status":true,"errorMessage":null,"statusCode":201},{"key":"2","status":true,"errorMessage":null,"statusCode":201}]}`))
	}))
	defer srv.Close()

	out, err := document_upload.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		objConn("documents", `[{"id":"1","content":"alpha"},{"id":"2","content":"beta"}]`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("out = %v", out)
	}
	if gotPath != "/indexes/products/docs/index" {
		t.Fatalf("path = %q", gotPath)
	}
	value, _ := gotBody["value"].([]interface{})
	if len(value) != 2 {
		t.Fatalf("body value = %v", value)
	}
	first, _ := value[0].(map[string]interface{})
	// Each document is wrapped with the write behaviour; default is upsert.
	if first["@search.action"] != "mergeOrUpload" {
		t.Fatalf("@search.action = %v", first["@search.action"])
	}
	if first["content"] != "alpha" {
		t.Fatalf("document fields lost: %v", first)
	}
	if out["count"] != 2 {
		t.Fatalf("count = %v", out["count"])
	}

	// A single bare object is accepted and wrapped.
	out, err = document_upload.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		objConn("documents", `{"id":"9"}`),
		strConn("action", "upload"),
	))
	if err != nil {
		t.Fatalf("Execute single: %v", err)
	}
	value, _ = gotBody["value"].([]interface{})
	first, _ = value[0].(map[string]interface{})
	if len(value) != 1 || first["@search.action"] != "upload" {
		t.Fatalf("single-doc body = %v", gotBody)
	}

	// An unknown action is a soft validation failure.
	out, err = document_upload.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		objConn("documents", `[{"id":"1"}]`),
		strConn("action", "replace"),
	))
	if err != nil {
		t.Fatalf("Execute bad action: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "not valid") {
		t.Fatalf("bad action result = %v", out)
	}
}

func TestDocumentUploadPartialFailure(t *testing.T) {
	// 207 Multi-Status: the per-document status array is the real verdict.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`{"value":[
			{"key":"1","status":true,"errorMessage":null,"statusCode":201},
			{"key":"2","status":false,"errorMessage":"Document is too large","statusCode":413},
			{"key":"3","status":false,"errorMessage":null,"statusCode":404}
		]}`))
	}))
	defer srv.Close()

	out, err := document_upload.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		objConn("documents", `[{"id":"1"},{"id":"2"},{"id":"3"}]`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("partial failure must soft-fail: %v", out)
	}
	msg := out["error"].(string)
	if !strings.Contains(msg, "2 of 3") || !strings.Contains(msg, "2 (Document is too large)") || !strings.Contains(msg, "3") {
		t.Fatalf("error = %q", msg)
	}
	// The per-document statuses still ride along for downstream handling.
	statuses, _ := out["results"].([]interface{})
	if len(statuses) != 3 || out["count"] != 1 {
		t.Fatalf("results = %v count = %v", statuses, out["count"])
	}
}

func TestDocumentDelete(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"value":[{"key":"a","status":true,"statusCode":200},{"key":"b","status":true,"statusCode":200}]}`))
	}))
	defer srv.Close()

	out, err := document_delete.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		strConn("key_field", "id"),
		strConn("keys", "a, b"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["count"] != 2 {
		t.Fatalf("out = %v", out)
	}
	value, _ := gotBody["value"].([]interface{})
	if len(value) != 2 {
		t.Fatalf("body value = %v", value)
	}
	first, _ := value[0].(map[string]interface{})
	if first["@search.action"] != "delete" || first["id"] != "a" {
		t.Fatalf("delete entry = %v", first)
	}

	// Blank keys after splitting is a soft validation failure.
	out, err = document_delete.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		strConn("key_field", "id"),
		strConn("keys", " , ,"),
	))
	if err != nil {
		t.Fatalf("Execute blank keys: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("blank keys result = %v", out)
	}
}

func TestDocumentGet(t *testing.T) {
	var gotURI, gotSelect string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		gotSelect = r.URL.Query().Get("$select")
		_, _ = w.Write([]byte(`{"id":"doc-1","content":"hello"}`))
	}))
	defer srv.Close()

	out, err := document_get.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		strConn("key", "doc-1"),
		strConn("select", "id,content"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "doc-1" {
		t.Fatalf("out = %v", out)
	}
	if !strings.Contains(gotURI, "/indexes/products/docs('doc-1')") {
		t.Fatalf("lookup URI = %q — the OData ('key') path is required", gotURI)
	}
	if gotSelect != "id,content" {
		t.Fatalf("$select = %q", gotSelect)
	}
	result, _ := out["result"].(map[string]interface{})
	if result["content"] != "hello" {
		t.Fatalf("result = %v", result)
	}
}

func TestDocumentCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/docs/$count") {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		// The service prefixes a UTF-8 BOM on this endpoint.
		_, _ = w.Write([]byte("\xef\xbb\xbf1729"))
	}))
	defer srv.Close()

	out, err := document_count.Execute(nil, nil, authInputs(srv.URL, strConn("index_name", "products")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["count"] != int64(1729) {
		t.Fatalf("out = %v", out)
	}
	if !strings.Contains(out["tool_result"].(string), "1729") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}

	// An auth failure surfaces the envelope as a soft failure with the key
	// redacted.
	srv403 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":{"code":"Forbidden","message":"Authorization failed for key ` + testKey + `"}}`))
	}))
	defer srv403.Close()
	out, err = document_count.Execute(nil, nil, authInputs(srv403.URL, strConn("index_name", "products")))
	if err != nil {
		t.Fatalf("Execute 403: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "Forbidden") {
		t.Fatalf("403 result = %v", out)
	}
	if strings.Contains(out["error"].(string), testKey) {
		t.Fatalf("API key leaked into error: %v", out["error"])
	}
}
