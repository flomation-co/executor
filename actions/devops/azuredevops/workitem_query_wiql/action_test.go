package devops_azuredevops_workitem_query_wiql_test

import (
	"encoding/json"
	core "flomation.app/automate/executor"
	action "flomation.app/automate/executor/actions/devops/azuredevops/workitem_query_wiql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// conns builds the action's inputs against a stub organisation. The credential
// block is positional-by-name, so it is spelled out rather than shared.
func conns(orgURL string, extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "organisation_url", Type: core.ConnectionTypeString, Value: orgURL + "/myorg"},
		{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Value: "test-pat"},
	}, extra...)
}

func TestWorkitemQueryWiql(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// api-version is mandatory on every call; its absence does not fail
		// cleanly against the real service, so assert it here.
		if r.URL.Query().Get("api-version") == "" {
			t.Error("api-version is missing from the request")
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/_apis/wit/wiql") {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["query"] != "SELECT [System.Id] FROM WorkItems" {
				t.Errorf("query = %v", body["query"])
			}
			// WIQL returns ONLY references — no field values, ever.
			_, _ = w.Write([]byte(`{"workItems":[{"id":42,"url":"u"},{"id":43,"url":"u"}]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/_apis/wit/workitemsbatch") {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body["ids"].([]interface{})) != 2 {
				t.Errorf("batch ids = %v", body["ids"])
			}
			_, _ = w.Write([]byte(`{"count":2,"value":[{"id":42,"fields":{"System.Title":"a"}},{"id":43,"fields":{"System.Title":"b"}}]}`))
			return
		}
		t.Errorf("unexpected path %q", r.URL.Path)
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "query", Type: core.ConnectionTypeText, Value: `SELECT [System.Id] FROM WorkItems`},
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: 50},
	))
	if err != nil {
		t.Fatalf("Execute returned a hard error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, error = %v", out["success"], out["error"])
	}
	// The references must have been hydrated — a bare id list is useless in a flow.
	if out["count"] != 2 {
		t.Fatalf("count = %v, want 2", out["count"])
	}
	items := out["results"].([]interface{})
	first := items[0].(map[string]interface{})
	if first["fields"] == nil {
		t.Error("the results were not hydrated via workitemsbatch")
	}
}

// TestWorkitemQueryWiqlAPIError pins the soft-failure contract: a provider error is data on
// the error port, never a hard Go error that would abort the flow run.
func TestWorkitemQueryWiqlAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"TF200016: The following project does not exist: MyProject."}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "query", Type: core.ConnectionTypeText, Value: `SELECT [System.Id] FROM WorkItems`},
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: 50},
	))
	if err != nil {
		t.Fatalf("a provider error must be soft, got a hard error: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("success = %v, want false", out["success"])
	}
	msg, _ := out["error"].(string)
	if !strings.Contains(msg, "TF200016") {
		t.Errorf("error = %q, want the service's own message surfaced", msg)
	}
	if strings.Contains(msg, "test-pat") {
		t.Errorf("the PAT leaked into the error: %q", msg)
	}
}

// TestWorkitemQueryWiqlSignInPage pins the 203 trap: a bad or expired PAT answers 203 with an
// HTML sign-in page rather than 401, so a plain 2xx check would call this a
// success and then fail on an HTML-to-JSON decode.
func TestWorkitemQueryWiqlSignInPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNonAuthoritativeInfo)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign In</body></html>"))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "query", Type: core.ConnectionTypeText, Value: `SELECT [System.Id] FROM WorkItems`},
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: 50},
	))
	if err != nil {
		t.Fatalf("hard error: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("a 203 sign-in page must fail, got success = %v", out["success"])
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "Personal Access Token") {
		t.Errorf("error = %q, want it to name the Personal Access Token", msg)
	}
}
