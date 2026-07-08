package intercom

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

func strConn(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}
func objConn(name, jsonStr string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: jsonStr}
}

func TestDoSetsBearerAndVersionHeaders(t *testing.T) {
	var gotAuth, gotVersion, gotAccept, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("Intercom-Version")
		gotAccept = r.Header.Get("Accept")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"type":"contact","id":"abc123","email":"jane@acme.com"}`))
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()

	a := Auth{Token: "TOK-XYZ", Region: "us"}
	obj, err := WriteObject(a, http.MethodPost, "/contacts", map[string]interface{}{"email": "jane@acme.com"}, nil)
	if err != nil {
		t.Fatalf("WriteObject: %v", err)
	}
	if gotAuth != "Bearer TOK-XYZ" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if gotVersion != "2.15" {
		t.Fatalf("Intercom-Version header = %q", gotVersion)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept header = %q", gotAccept)
	}
	// Intercom bodies are plain objects — no envelope wrapping.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(gotBody), &parsed); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if parsed["email"] != "jane@acme.com" {
		t.Fatalf("request body = %s", gotBody)
	}
	if obj["id"] != "abc123" {
		t.Fatalf("decoded id = %v", obj["id"])
	}
	if got := ResourceResult(obj, "")["id"]; got != "abc123" {
		t.Fatalf("ResourceResult id = %v", got)
	}
}

func TestCheckResponseSurfacesErrorList(t *testing.T) {
	body := `{"type":"error.list","request_id":"r1","errors":[{"code":"not_found","message":"Contact Not Found"},{"code":"parameter_invalid","message":"bad role"}]}`
	err := CheckResponse(&APIResponse{StatusCode: 404, Body: []byte(body)})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not_found: Contact Not Found") {
		t.Fatalf("first error not surfaced as code: message, got %v", msg)
	}
	if !strings.Contains(msg, "parameter_invalid: bad role") {
		t.Fatalf("errors not joined, got %v", msg)
	}
	if !strings.Contains(msg, "404") {
		t.Fatalf("HTTP status missing, got %v", msg)
	}
}

func TestRedactAuth(t *testing.T) {
	a := Auth{Token: "SUPERSECRET"}
	if msg := redactAuth(a, "boom with token SUPERSECRET in it"); strings.Contains(msg, "SUPERSECRET") {
		t.Fatalf("token not redacted: %s", msg)
	}
}

func TestGetAuthRegionMapsHost(t *testing.T) {
	inputs := []*core.Connection{
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		strConn("region", "eu"),
	}
	a, err := GetAuth(inputs)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if a.BaseURL() != "https://api.eu.intercom.io" {
		t.Fatalf("eu base = %q", a.BaseURL())
	}
	// Empty region defaults to us.
	a, err = GetAuth(inputs[:1])
	if err != nil {
		t.Fatalf("GetAuth default: %v", err)
	}
	if a.Region != "us" || a.BaseURL() != "https://api.intercom.io" {
		t.Fatalf("default region = %q base = %q", a.Region, a.BaseURL())
	}
}

func TestListAllFollowsStartingAfterCursor(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("starting_after") == "" {
			// First page → pages.next is an object carrying the cursor.
			_, _ = w.Write([]byte(`{"type":"list","data":[{"id":"1"},{"id":"2"}],"pages":{"type":"pages","page":1,"next":{"page":2,"starting_after":"cur2"}}}`))
		} else {
			// Second page → last (next null).
			if got := r.URL.Query().Get("starting_after"); got != "cur2" {
				t.Errorf("starting_after = %q", got)
			}
			_, _ = w.Write([]byte(`{"type":"list","data":[{"id":"3"}],"pages":{"type":"pages","page":2,"next":null}}`))
		}
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()

	items, err := ListAll(Auth{Token: "t", Region: "us"}, "/contacts", nil, "data", 0, true)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items across pages, got %d", len(items))
	}
	if calls != 2 {
		t.Fatalf("expected 2 page fetches, got %d", calls)
	}

	// returnAll=false → single page only.
	calls = 0
	single, err := ListAll(Auth{Token: "t", Region: "us"}, "/contacts", nil, "data", 0, false)
	if err != nil {
		t.Fatalf("ListAll single: %v", err)
	}
	if len(single) != 2 || calls != 1 {
		t.Fatalf("single page: got %d items in %d calls", len(single), calls)
	}
}

func TestListPageArrayKeyFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Envelope keyed "conversations" — neither the requested key nor "data".
		_, _ = w.Write([]byte(`{"type":"conversation.list","conversations":[{"id":"9"}],"pages":{"next":null}}`))
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()

	items, cursor, err := ListPage(Auth{Token: "t", Region: "us"}, "/conversations", nil, "wrong_key")
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if len(items) != 1 || cursor != "" {
		t.Fatalf("fallback extraction: %d items, cursor %q", len(items), cursor)
	}
}

func TestSearchAllPaginatesInsideBody(t *testing.T) {
	calls := 0
	var firstQuery map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost {
			t.Errorf("search method = %s", r.Method)
		}
		var body map[string]interface{}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		pagination, _ := body["pagination"].(map[string]interface{})
		if pagination == nil {
			t.Errorf("no pagination in body: %s", b)
		}
		if calls == 1 {
			firstQuery, _ = body["query"].(map[string]interface{})
			if pagination["starting_after"] != nil {
				t.Errorf("first page carries a cursor: %v", pagination["starting_after"])
			}
			_, _ = w.Write([]byte(`{"type":"list","data":[{"id":"1"}],"pages":{"next":{"page":2,"starting_after":"sc2"}}}`))
		} else {
			if got, _ := pagination["starting_after"].(string); got != "sc2" {
				t.Errorf("body cursor = %v", pagination["starting_after"])
			}
			_, _ = w.Write([]byte(`{"type":"list","data":[{"id":"2"}],"pages":{"next":null}}`))
		}
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()

	dsl := map[string]interface{}{"field": "email", "operator": "~", "value": "@acme.com"}
	items, err := SearchAll(Auth{Token: "t", Region: "us"}, "/contacts/search", dsl, "data", 0, true)
	if err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	if len(items) != 2 || calls != 2 {
		t.Fatalf("got %d items in %d calls", len(items), calls)
	}
	if firstQuery == nil || firstQuery["field"] != "email" {
		t.Fatalf("query DSL not sent in body: %v", firstQuery)
	}
}

func TestDeleteWithBodySendsJSON(t *testing.T) {
	var gotMethod, gotBody, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()

	err := DeleteWithBody(Auth{Token: "t", Region: "us"}, "/conversations/1/tags/2", map[string]interface{}{"admin_id": "5"})
	if err != nil {
		t.Fatalf("DeleteWithBody: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %s", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content type = %q", gotContentType)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(gotBody), &parsed); err != nil || parsed["admin_id"] != "5" {
		t.Fatalf("DELETE body = %q", gotBody)
	}
}

func TestEmptyBodyDecodesToEmptyObject(t *testing.T) {
	// POST /events replies 202 with no content — must not error on decode.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()

	obj, err := WriteObject(Auth{Token: "t", Region: "us"}, http.MethodPost, "/events", map[string]interface{}{"event_name": "signed-up"}, nil)
	if err != nil {
		t.Fatalf("WriteObject on 202 empty body: %v", err)
	}
	if len(obj) != 0 {
		t.Fatalf("expected empty object, got %v", obj)
	}
}

func TestClampLimit(t *testing.T) {
	if got := ClampLimit(0, false); got != DefaultPageLimit {
		t.Fatalf("unset limit = %d", got)
	}
	if got := ClampLimit(500, true); got != MaxPageLimit {
		t.Fatalf("oversized limit = %d", got)
	}
	if got := ClampLimit(42, true); got != 42 {
		t.Fatalf("in-range limit = %d", got)
	}
}

func TestBuildSearchQuery(t *testing.T) {
	// Simple single filter.
	q, err := BuildSearchQuery([]*core.Connection{
		strConn("field", "email"), strConn("operator", "="), strConn("value", "jane@acme.com"),
	})
	if err != nil {
		t.Fatalf("simple filter: %v", err)
	}
	if q["field"] != "email" || q["operator"] != "=" || q["value"] != "jane@acme.com" {
		t.Fatalf("simple filter = %v", q)
	}

	// IN splits the value on commas into an array.
	q, err = BuildSearchQuery([]*core.Connection{
		strConn("field", "role"), strConn("operator", "IN"), strConn("value", "user, lead"),
	})
	if err != nil {
		t.Fatalf("IN filter: %v", err)
	}
	vals, ok := q["value"].([]interface{})
	if !ok || len(vals) != 2 || vals[1] != "lead" {
		t.Fatalf("IN value = %v", q["value"])
	}

	// query_json wins verbatim over field/operator/value.
	q, err = BuildSearchQuery([]*core.Connection{
		strConn("field", "email"), strConn("operator", "="), strConn("value", "x"),
		objConn("query_json", `{"operator":"AND","value":[{"field":"role","operator":"=","value":"user"}]}`),
	})
	if err != nil {
		t.Fatalf("query_json: %v", err)
	}
	if q["operator"] != "AND" {
		t.Fatalf("query_json did not override: %v", q)
	}

	// Invalid operator is rejected.
	if _, err = BuildSearchQuery([]*core.Connection{
		strConn("field", "email"), strConn("operator", ">="), strConn("value", "1"),
	}); err == nil {
		t.Fatal("expected error for unsupported operator")
	}
}

func TestParseUnixTime(t *testing.T) {
	if n, err := ParseUnixTime("1720000000"); err != nil || n != 1720000000 {
		t.Fatalf("epoch string: %d, %v", n, err)
	}
	if n, err := ParseUnixTime("2026-07-08T09:00:00Z"); err != nil || n != 1783501200 {
		t.Fatalf("RFC3339: %d, %v", n, err)
	}
	if n, err := ParseUnixTime("2026-07-08"); err != nil || n != 1783468800 {
		t.Fatalf("bare date: %d, %v", n, err)
	}
	if _, err := ParseUnixTime("next tuesday"); err == nil {
		t.Fatal("expected error for unparseable date")
	}
	// A 13-digit JS-style Date.now() millisecond epoch is converted to seconds
	// rather than taken verbatim (which would be year ~58,000 — silently
	// swallowed by Intercom's async event validation).
	if n, err := ParseUnixTime("1783534378000"); err != nil || n != 1783534378 {
		t.Fatalf("ms epoch: %d, %v", n, err)
	}
	if n, err := ParseUnixTime("-1783534378000"); err != nil || n != -1783534378 {
		t.Fatalf("negative ms epoch: %d, %v", n, err)
	}
	// Pre-1970 seconds epochs still pass through untouched.
	if n, err := ParseUnixTime("-86400"); err != nil || n != -86400 {
		t.Fatalf("negative seconds epoch: %d, %v", n, err)
	}
}

func TestSetIntIfPresent(t *testing.T) {
	// Absent and blank inputs are omitted, not errors.
	body := map[string]interface{}{}
	if err := SetIntIfPresent(body, []*core.Connection{}, "size", "size"); err != nil {
		t.Fatalf("absent: %v", err)
	}
	if err := SetIntIfPresent(body, []*core.Connection{strConn("size", "  ")}, "size", "size"); err != nil {
		t.Fatalf("blank: %v", err)
	}
	if _, ok := body["size"]; ok {
		t.Fatalf("absent/blank input set the field: %v", body)
	}

	// Whole-number strings parse.
	if err := SetIntIfPresent(body, []*core.Connection{strConn("size", "50")}, "size", "size"); err != nil || body["size"] != 50 {
		t.Fatalf("int string: %v %v", body["size"], err)
	}

	// Integer-typed inputs pass through Number().
	intConn := &core.Connection{Name: "monthly_spend", Type: core.ConnectionTypeInteger, Value: int64(490)}
	if err := SetIntIfPresent(body, []*core.Connection{intConn}, "monthly_spend", "monthly_spend"); err != nil || body["monthly_spend"] != 490 {
		t.Fatalf("integer input: %v %v", body["monthly_spend"], err)
	}

	// Float strings (an upstream billing node's 1499.99) truncate like
	// Intercom itself documents for monthly_spend.
	if err := SetIntIfPresent(body, []*core.Connection{strConn("monthly_spend", "1499.99")}, "monthly_spend", "monthly_spend"); err != nil || body["monthly_spend"] != 1499 {
		t.Fatalf("float string: %v %v", body["monthly_spend"], err)
	}

	// Present-but-unparseable values must ERROR, never be silently dropped.
	for _, bad := range []string{"1,200", "n/a", "abc"} {
		if err := SetIntIfPresent(body, []*core.Connection{strConn("size", bad)}, "size", "size"); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}

	// The same holds when the unparseable value landed in an Integer-typed
	// input (Connection.Number() and .String() both refuse it).
	badInt := &core.Connection{Name: "size", Type: core.ConnectionTypeInteger, Value: "1,200"}
	if err := SetIntIfPresent(body, []*core.Connection{badInt}, "size", "size"); err == nil {
		t.Fatal("expected error for unparseable Integer-typed value")
	}
}
