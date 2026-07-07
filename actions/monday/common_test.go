package monday_common

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

func TestGraphQLSendsBearerAndQuery(t *testing.T) {
	var gotAuth, gotVersion string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("API-Version")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"data":{"create_board":{"id":"123","name":"B"}}}`))
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()

	data, err := GraphQL(Auth{Token: "TK"}, "mutation { create_board { id } }", map[string]interface{}{"name": "B"})
	if err != nil {
		t.Fatalf("GraphQL: %v", err)
	}
	if gotAuth != "Bearer TK" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotVersion != APIVersion {
		t.Fatalf("api-version = %q", gotVersion)
	}
	if gotBody["query"] == nil || gotBody["variables"] == nil {
		t.Fatalf("body missing query/variables: %v", gotBody)
	}
	obj := ObjectField(data, "create_board")
	if obj["id"] != "123" {
		t.Fatalf("create_board id = %v", obj["id"])
	}
	if ResourceResult(obj, "")["id"] != "123" {
		t.Fatal("ResourceResult should read id")
	}
}

func TestGraphQLSurfacesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GraphQL returns 200 even on error.
		_, _ = w.Write([]byte(`{"errors":[{"message":"Board not found"}]}`))
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()

	_, err := GraphQL(Auth{Token: "TK"}, "query {}", nil)
	if err == nil || !strings.Contains(err.Error(), "Board not found") {
		t.Fatalf("expected GraphQL error surfaced, got %v", err)
	}
}

func TestGraphQLUnauthorised(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	defer SetBaseForTest(srv.URL)()
	if _, err := GraphQL(Auth{Token: "TK"}, "query {}", nil); err == nil || !strings.Contains(err.Error(), "unauthorised") {
		t.Fatalf("expected unauthorised error, got %v", err)
	}
}

func TestValidateJSON(t *testing.T) {
	good := &core.Connection{Name: "value", Type: core.ConnectionTypeObject, Value: `{"label":"Done"}`}
	v, err := ValidateJSON("value", []*core.Connection{good})
	if err != nil || v != `{"label":"Done"}` {
		t.Fatalf("valid json: v=%q err=%v", v, err)
	}
	bad := &core.Connection{Name: "value", Type: core.ConnectionTypeObject, Value: `{not json`}
	if _, err := ValidateJSON("value", []*core.Connection{bad}); err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestRedactAuth(t *testing.T) {
	if msg := redactAuth(Auth{Token: "SECRET"}, "fail SECRET here"); strings.Contains(msg, "SECRET") {
		t.Fatalf("token not redacted: %s", msg)
	}
}
