package asana_common

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestDoWrapsBodyAndSetsBearer(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"data":{"gid":"123","name":"T"}}`))
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()

	a := Auth{Token: "PAT-XYZ"}
	obj, err := WriteObject(a, http.MethodPost, "/tasks", map[string]interface{}{"name": "T"}, url.Values{})
	if err != nil {
		t.Fatalf("WriteObject: %v", err)
	}
	if gotAuth != "Bearer PAT-XYZ" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	// Body must be wrapped in a data envelope.
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(gotBody), &parsed); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if _, ok := parsed["data"]; !ok {
		t.Fatalf("request body not wrapped in data envelope: %s", gotBody)
	}
	if obj["gid"] != "123" {
		t.Fatalf("decoded gid = %v", obj["gid"])
	}
	if got := ResourceResult(obj, "")["id"]; got != "123" {
		t.Fatalf("ResourceResult id = %v (should read gid)", got)
	}
}

func TestListAllFollowsOffsetCursor(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("offset") == "" {
			// First page → has next_page.offset.
			_, _ = w.Write([]byte(`{"data":[{"gid":"1"},{"gid":"2"}],"next_page":{"offset":"tok2","uri":"x"}}`))
		} else {
			// Second page → last (next_page null).
			_, _ = w.Write([]byte(`{"data":[{"gid":"3"}],"next_page":null}`))
		}
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()

	items, err := ListAll(Auth{Token: "t"}, "/tasks", url.Values{}, 0, true)
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
	single, err := ListAll(Auth{Token: "t"}, "/tasks", url.Values{}, 0, false)
	if err != nil {
		t.Fatalf("ListAll single: %v", err)
	}
	if len(single) != 2 || calls != 1 {
		t.Fatalf("single page: got %d items in %d calls", len(single), calls)
	}
}

func TestCheckResponseSurfacesAsanaErrors(t *testing.T) {
	err := CheckResponse(&APIResponse{StatusCode: 400, Body: []byte(`{"errors":[{"message":"workspace: Unknown object"}]}`)})
	if err == nil || !strings.Contains(err.Error(), "workspace: Unknown object") {
		t.Fatalf("expected asana error surfaced, got %v", err)
	}
}

func TestRedactAuth(t *testing.T) {
	a := Auth{Token: "SUPERSECRET"}
	if msg := redactAuth(a, "boom with token SUPERSECRET in it"); strings.Contains(msg, "SUPERSECRET") {
		t.Fatalf("token not redacted: %s", msg)
	}
}

func TestMergeAdditionalFieldsOverrides(t *testing.T) {
	body := map[string]interface{}{"name": "first"}
	if err := MergeAdditionalFields(body, []*core.Connection{objConn("additional_fields", `{"name":"override","color":"dark-green"}`)}); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if body["name"] != "override" || body["color"] != "dark-green" {
		t.Fatalf("merge result = %+v", body)
	}
}

func TestSetStringListIfPresent(t *testing.T) {
	body := map[string]interface{}{}
	SetStringListIfPresent(body, []*core.Connection{strConn("projects", "a, b ,c")}, "projects", "projects")
	got, ok := body["projects"].([]string)
	if !ok || len(got) != 3 || got[1] != "b" {
		t.Fatalf("string list = %v", body["projects"])
	}
}
