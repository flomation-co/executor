// Tests for the index resource group: index_create (create-or-update PUT,
// name injection, If-None-Match/412 on only_if_missing), index_get (+404),
// index_get_all ($select, value unwrap), index_delete (DELETE 204) and
// index_stats (typed count/size outputs).
//
// This is an EXTERNAL test package (azureaisearch_test) so it can import the
// sibling action packages (which import azureaisearch) without a cycle. The
// input constructors below are shared by documents_test.go and search_test.go
// — declare them here only.
package azureaisearch_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"

	index_create "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/index_create"
	index_delete "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/index_delete"
	index_get "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/index_get"
	index_get_all "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/index_get_all"
	index_stats "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/index_stats"
)

const testKey = "admin-key-XYZ"

func strConn(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}
func secretConn(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeSecret, Value: val}
}
func textConn(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: val}
}
func objConn(name, jsonStr string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: jsonStr}
}
func boolConn(name string, val bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: val}
}
func intConn(name string, val int) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: val}
}

// authInputs points the credential block at an httptest server via the Custom
// Endpoint input (which wins over service_name by design).
func authInputs(srvURL string, extra ...*core.Connection) []*core.Connection {
	base := []*core.Connection{
		strConn("endpoint", srvURL),
		secretConn("api_key", testKey),
	}
	return append(base, extra...)
}

func TestIndexCreate(t *testing.T) {
	var gotMethod, gotPath, gotIfNoneMatch, gotVersion string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotIfNoneMatch = r.Header.Get("If-None-Match")
		gotVersion = r.URL.Query().Get("api-version")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"name":"products","fields":[{"name":"id","type":"Edm.String","key":true}]}`))
	}))
	defer srv.Close()

	out, err := index_create.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		objConn("definition", `{"name":"WRONG","fields":[{"name":"id","type":"Edm.String","key":true}]}`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if gotMethod != http.MethodPut || gotPath != "/indexes/products" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if gotVersion == "" {
		t.Fatalf("api-version missing")
	}
	if gotIfNoneMatch != "" {
		t.Fatalf("If-None-Match sent without only_if_missing: %q", gotIfNoneMatch)
	}
	// The URL names the index — the input wins over the definition's "name".
	if gotBody["name"] != "products" {
		t.Fatalf(`body name = %v, want "products"`, gotBody["name"])
	}
	if out["id"] != "products" {
		t.Fatalf("id = %v", out["id"])
	}
	if !strings.Contains(out["tool_result"].(string), "Created") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}

	// A non-object definition is a soft validation failure.
	out, err = index_create.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		objConn("definition", `[1,2]`),
	))
	if err != nil {
		t.Fatalf("Execute bad definition: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "JSON object") {
		t.Fatalf("bad definition result = %v", out)
	}
}

func TestIndexCreateOnlyIfMissing(t *testing.T) {
	var gotIfNoneMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfNoneMatch = r.Header.Get("If-None-Match")
		// The index exists, so the conditional create fails the precondition.
		w.WriteHeader(http.StatusPreconditionFailed)
	}))
	defer srv.Close()

	out, err := index_create.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		objConn("definition", `{"fields":[]}`),
		boolConn("only_if_missing", true),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotIfNoneMatch != "*" {
		t.Fatalf("If-None-Match = %q, want *", gotIfNoneMatch)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "already exists") {
		t.Fatalf("412 result = %v", out)
	}
}

func TestIndexGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/indexes/products" || r.Method != http.MethodGet {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"name":"products","fields":[{"name":"id"},{"name":"content"}]}`))
	}))
	defer srv.Close()

	out, err := index_get.Execute(nil, nil, authInputs(srv.URL, strConn("index_name", "products")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "products" {
		t.Fatalf("out = %v", out)
	}
	if !strings.Contains(out["tool_result"].(string), "2 fields") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}

	// A missing index surfaces the Azure envelope as a soft failure.
	srv404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":{"code":"ResourceNotFound","message":"no such index"}}`))
	}))
	defer srv404.Close()
	out, err = index_get.Execute(nil, nil, authInputs(srv404.URL, strConn("index_name", "nope")))
	if err != nil {
		t.Fatalf("Execute 404: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "ResourceNotFound") {
		t.Fatalf("404 result = %v", out)
	}
}

func TestIndexGetAll(t *testing.T) {
	var gotSelect string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSelect = r.URL.Query().Get("$select")
		_, _ = w.Write([]byte(`{"value":[{"name":"a"},{"name":"b"},{"name":"c"}]}`))
	}))
	defer srv.Close()

	out, err := index_get_all.Execute(nil, nil, authInputs(srv.URL, strConn("select", "name")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("out = %v", out)
	}
	if gotSelect != "name" {
		t.Fatalf("$select = %q", gotSelect)
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 3 || out["count"] != 3 {
		t.Fatalf("results = %v count = %v", results, out["count"])
	}
}

func TestIndexDelete(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()

	out, err := index_delete.Execute(nil, nil, authInputs(srv.URL, strConn("index_name", "products")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/indexes/products" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if out["success"] != true || out["id"] != "products" {
		t.Fatalf("out = %v", out)
	}

	// Missing index_name is a soft validation failure before any request.
	out, err = index_delete.Execute(nil, nil, authInputs(srv.URL))
	if err != nil {
		t.Fatalf("Execute missing name: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "index_name") {
		t.Fatalf("missing name result = %v", out)
	}
}

func TestIndexStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/indexes/products/stats" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"documentCount":1234,"storageSize":567890,"vectorIndexSize":1024}`))
	}))
	defer srv.Close()

	out, err := index_stats.Execute(nil, nil, authInputs(srv.URL, strConn("index_name", "products")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("out = %v", out)
	}
	if out["document_count"] != int64(1234) {
		t.Fatalf("document_count = %v (%T)", out["document_count"], out["document_count"])
	}
	if out["storage_size"] != int64(567890) {
		t.Fatalf("storage_size = %v", out["storage_size"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["vectorIndexSize"] != float64(1024) {
		t.Fatalf("result = %v", result)
	}
}
