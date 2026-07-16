// Tests for the search action: keyword default (blank text becomes "*",
// count:true always sent, top clamped), vector mode (vectorQueries body, no
// search text), hybrid (both), semantic configuration (queryType switch),
// @odata.count as the count output, and the vector-validation soft failures.
// Input constructors live in indexes_test.go.
package azureaisearch_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	search "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/search"
)

// searchServer records the POST body and answers with two hits and a larger
// total, so the count-vs-page distinction is observable.
func searchServer(t *testing.T, gotBody *map[string]interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/docs/search") {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, gotBody)
		_, _ = w.Write([]byte(`{"@odata.count":42,"value":[{"@search.score":1.2,"id":"1"},{"@search.score":0.8,"id":"2"}]}`))
	}))
}

func TestSearchKeywordDefaults(t *testing.T) {
	var gotBody map[string]interface{}
	srv := searchServer(t, &gotBody)
	defer srv.Close()

	out, err := search.Execute(nil, nil, authInputs(srv.URL, strConn("index_name", "products")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("out = %v", out)
	}
	// Blank text matches everything; count is always requested; top defaults.
	if gotBody["search"] != "*" {
		t.Fatalf("search = %v", gotBody["search"])
	}
	if gotBody["count"] != true {
		t.Fatalf("count = %v — $count must always be requested", gotBody["count"])
	}
	if gotBody["top"] != float64(50) {
		t.Fatalf("top = %v", gotBody["top"])
	}
	if _, has := gotBody["vectorQueries"]; has {
		t.Fatalf("keyword search must not carry vectorQueries: %v", gotBody)
	}
	// count carries the TOTAL (@odata.count), results the returned page.
	if out["count"] != 42 {
		t.Fatalf("count = %v, want the @odata.count total", out["count"])
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 2 {
		t.Fatalf("results = %v", results)
	}
	if !strings.Contains(out["tool_result"].(string), "42") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}
}

func TestSearchVectorMode(t *testing.T) {
	var gotBody map[string]interface{}
	srv := searchServer(t, &gotBody)
	defer srv.Close()

	out, err := search.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		strConn("mode", "vector"),
		textConn("vector", "[0.1, -0.2, 0.3]"),
		strConn("vector_field", "embedding"),
		intConn("k", 5),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("out = %v", out)
	}
	// A pure vector query carries no search text — the embedding is the query.
	if _, has := gotBody["search"]; has {
		t.Fatalf("vector search must not carry search text: %v", gotBody)
	}
	vqs, _ := gotBody["vectorQueries"].([]interface{})
	if len(vqs) != 1 {
		t.Fatalf("vectorQueries = %v", gotBody["vectorQueries"])
	}
	vq, _ := vqs[0].(map[string]interface{})
	if vq["kind"] != "vector" || vq["fields"] != "embedding" || vq["k"] != float64(5) {
		t.Fatalf("vectorQuery = %v", vq)
	}
	vec, _ := vq["vector"].([]interface{})
	if len(vec) != 3 || vec[1] != float64(-0.2) {
		t.Fatalf("vector = %v", vec)
	}

	// Vector mode without a vector is a soft validation failure.
	out, err = search.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		strConn("mode", "vector"),
	))
	if err != nil {
		t.Fatalf("Execute missing vector: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "vector is required") {
		t.Fatalf("missing vector result = %v", out)
	}

	// A malformed vector is too.
	out, err = search.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		strConn("mode", "vector"),
		textConn("vector", `["not","numbers"]`),
	))
	if err != nil {
		t.Fatalf("Execute bad vector: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "JSON array of numbers") {
		t.Fatalf("bad vector result = %v", out)
	}
}

func TestSearchHybridAndSemantic(t *testing.T) {
	var gotBody map[string]interface{}
	srv := searchServer(t, &gotBody)
	defer srv.Close()

	out, err := search.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		strConn("mode", "hybrid"),
		strConn("search_text", "wireless headphones"),
		textConn("vector", "[0.5, 0.5]"),
		strConn("filter", "category eq 'audio'"),
		strConn("select", "id,title"),
		strConn("order_by", "rating desc"),
		intConn("top", 3),
		strConn("semantic_configuration", "my-semantic"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("out = %v", out)
	}
	// Hybrid fuses keyword and vector: both travel in one request.
	if gotBody["search"] != "wireless headphones" {
		t.Fatalf("search = %v", gotBody["search"])
	}
	if _, has := gotBody["vectorQueries"]; !has {
		t.Fatalf("hybrid search must carry vectorQueries: %v", gotBody)
	}
	vqs, _ := gotBody["vectorQueries"].([]interface{})
	vq, _ := vqs[0].(map[string]interface{})
	// vector_field and k fall back to their defaults when unset.
	if vq["fields"] != "content_vector" || vq["k"] != float64(10) {
		t.Fatalf("vectorQuery defaults = %v", vq)
	}
	if gotBody["filter"] != "category eq 'audio'" || gotBody["select"] != "id,title" || gotBody["orderby"] != "rating desc" {
		t.Fatalf("filters = %v", gotBody)
	}
	if gotBody["top"] != float64(3) {
		t.Fatalf("top = %v", gotBody["top"])
	}
	// A named semantic configuration switches on the semantic ranker.
	if gotBody["queryType"] != "semantic" || gotBody["semanticConfiguration"] != "my-semantic" {
		t.Fatalf("semantic wiring = %v", gotBody)
	}
	if !strings.Contains(out["tool_result"].(string), "semantic ranking") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}
}

func TestSearchAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"code":"InvalidRequestParameter","message":"Unknown field 'nope' in vector query"}}`))
	}))
	defer srv.Close()

	out, err := search.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		strConn("mode", "vector"),
		textConn("vector", "[0.1]"),
		strConn("vector_field", "nope"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "InvalidRequestParameter") {
		t.Fatalf("error result = %v", out)
	}

	// An invalid mode never reaches the wire.
	out, err = search.Execute(nil, nil, authInputs(srv.URL,
		strConn("index_name", "products"),
		strConn("mode", "fuzzy"),
	))
	if err != nil {
		t.Fatalf("Execute bad mode: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), `mode "fuzzy" is not valid`) {
		t.Fatalf("bad mode result = %v", out)
	}
}
