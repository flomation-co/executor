package trello_common

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// strConn / boolConn / objConn build the *core.Connection inputs an action
// receives, mirroring how the engine passes resolved values.
func strConn(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}
func boolConn(name string, val bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: val}
}
func objConn(name, json string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: json}
}

func authConns() []*core.Connection {
	return []*core.Connection{strConn("api_key", "KEY123"), strConn("api_token", "TOK456")}
}

// newServer stands up an httptest server, points the package base at it, and
// captures the last request's method, path and query for assertions.
func newServer(t *testing.T, status int, body string) (*httptest.Server, *http.Request, *url.Values) {
	t.Helper()
	var lastReq http.Request
	var lastQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastReq = *r
		lastQuery = r.URL.Query()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	restore := SetBaseForTest(srv.URL)
	t.Cleanup(func() { restore(); srv.Close() })
	return srv, &lastReq, &lastQuery
}

func TestGetAuth(t *testing.T) {
	a, err := GetAuth(authConns())
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if a.Key != "KEY123" || a.Token != "TOK456" {
		t.Fatalf("unexpected auth %+v", a)
	}
	if _, err := GetAuth([]*core.Connection{strConn("api_key", "k")}); err == nil {
		t.Fatal("expected error when token missing")
	}
}

func TestDoAppendsCredentials(t *testing.T) {
	_, lastReq, lastQuery := newServer(t, 200, `{"id":"abc"}`)
	a := Auth{Key: "KEY123", Token: "TOK456"}
	params := url.Values{}
	params.Set("name", "Hello World")
	if _, err := Do(a, http.MethodPost, "/boards", params); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if lastReq.Method != http.MethodPost {
		t.Fatalf("method = %s", lastReq.Method)
	}
	if got := lastQuery.Get("key"); got != "KEY123" {
		t.Fatalf("key = %q", got)
	}
	if got := lastQuery.Get("token"); got != "TOK456" {
		t.Fatalf("token = %q", got)
	}
	if got := lastQuery.Get("name"); got != "Hello World" {
		t.Fatalf("name = %q (spaces should be encoded/decoded cleanly)", got)
	}
	if lastReq.URL.Path != "/boards" {
		t.Fatalf("path = %q", lastReq.URL.Path)
	}
}

func TestCheckResponseSurfacesBody(t *testing.T) {
	err := CheckResponse(&APIResponse{StatusCode: 400, Body: []byte("invalid value for name")})
	if err == nil || !strings.Contains(err.Error(), "invalid value for name") {
		t.Fatalf("expected body in error, got %v", err)
	}
	if CheckResponse(&APIResponse{StatusCode: 200}) != nil {
		t.Fatal("200 should not error")
	}
}

func TestRedactAuth(t *testing.T) {
	a := Auth{Key: "SECRETKEY", Token: "SECRETTOK"}
	msg := redactAuth(a, "failed calling ...?key=SECRETKEY&token=SECRETTOK")
	if strings.Contains(msg, "SECRETKEY") || strings.Contains(msg, "SECRETTOK") {
		t.Fatalf("credentials not redacted: %s", msg)
	}
}

func TestSetBoolIfSet(t *testing.T) {
	p := url.Values{}
	SetBoolIfSet(p, []*core.Connection{boolConn("closed", true)}, "closed", "closed")
	if p.Get("closed") != "true" {
		t.Fatalf("closed = %q", p.Get("closed"))
	}
	// Unset -> omitted.
	p2 := url.Values{}
	SetBoolIfSet(p2, []*core.Connection{}, "closed", "closed")
	if _, ok := p2["closed"]; ok {
		t.Fatal("unset bool should be omitted")
	}
}

func TestMergeAdditionalFields(t *testing.T) {
	p := url.Values{}
	p.Set("name", "first")
	conns := []*core.Connection{objConn("additional_fields", `{"name":"override","prefs_permissionLevel":"org","n":3,"b":true}`)}
	if err := MergeAdditionalFields(p, conns); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if p.Get("name") != "override" {
		t.Fatalf("additional_fields should win, got %q", p.Get("name"))
	}
	if p.Get("prefs_permissionLevel") != "org" {
		t.Fatalf("prefs = %q", p.Get("prefs_permissionLevel"))
	}
	if p.Get("n") != "3" {
		t.Fatalf("number should stringify without .0, got %q", p.Get("n"))
	}
	if p.Get("b") != "true" {
		t.Fatalf("bool = %q", p.Get("b"))
	}
	// Non-object -> error.
	if err := MergeAdditionalFields(url.Values{}, []*core.Connection{objConn("additional_fields", `["a"]`)}); err == nil {
		t.Fatal("expected error for non-object additional_fields")
	}
}

func TestResourceAndListResult(t *testing.T) {
	r := ResourceResult(map[string]interface{}{"id": "xyz", "name": "b"}, "done")
	if r["id"] != "xyz" || r["success"] != true {
		t.Fatalf("ResourceResult = %+v", r)
	}
	l := ListResult([]interface{}{1, 2, 3}, "listed")
	if l["count"] != 3 || l["total"] != 3 {
		t.Fatalf("ListResult = %+v", l)
	}
	e := ErrorResult("bad")
	if e["success"] != false || e["error"] != "bad" {
		t.Fatalf("ErrorResult = %+v", e)
	}
}
