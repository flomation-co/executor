package devops_azuredevops_workitem_get_test

import (
	core "flomation.app/automate/executor"
	action "flomation.app/automate/executor/actions/devops/azuredevops/workitem_get"
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

func TestWorkitemGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// api-version is mandatory on every call; its absence does not fail
		// cleanly against the real service, so assert it here.
		if r.URL.Query().Get("api-version") == "" {
			t.Error("api-version is missing from the request")
		}
		w.Header().Set("Content-Type", "application/json")
		// No project supplied: work items are addressable organisation-wide.
		if !strings.HasSuffix(r.URL.Path, "/myorg/_apis/wit/workitems/42") {
			t.Errorf("path = %q, want the org-scoped form", r.URL.Path)
		}
		if r.URL.Query().Get("$expand") != "relations" {
			t.Errorf("$expand = %q", r.URL.Query().Get("$expand"))
		}
		_, _ = w.Write([]byte(`{"id":42,"fields":{"System.Title":"Checkout is broken"},"url":"u"}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "work_item_id", Type: core.ConnectionTypeInteger, Value: 42},
		&core.Connection{Name: "expand", Type: core.ConnectionTypeString, Value: "relations"},
	))
	if err != nil {
		t.Fatalf("Execute returned a hard error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, error = %v", out["success"], out["error"])
	}
	if out["id"] != "42" {
		t.Errorf("id = %v", out["id"])
	}
}

// TestWorkitemGetAPIError pins the soft-failure contract: a provider error is data on
// the error port, never a hard Go error that would abort the flow run.
func TestWorkitemGetAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"TF200016: The following project does not exist: MyProject."}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "work_item_id", Type: core.ConnectionTypeInteger, Value: 42},
		&core.Connection{Name: "expand", Type: core.ConnectionTypeString, Value: "relations"},
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

// TestWorkitemGetSignInPage pins the 203 trap: a bad or expired PAT answers 203 with an
// HTML sign-in page rather than 401, so a plain 2xx check would call this a
// success and then fail on an HTML-to-JSON decode.
func TestWorkitemGetSignInPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNonAuthoritativeInfo)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign In</body></html>"))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "work_item_id", Type: core.ConnectionTypeInteger, Value: 42},
		&core.Connection{Name: "expand", Type: core.ConnectionTypeString, Value: "relations"},
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
