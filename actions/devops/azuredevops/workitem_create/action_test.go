package devops_azuredevops_workitem_create_test

import (
	"encoding/json"
	core "flomation.app/automate/executor"
	action "flomation.app/automate/executor/actions/devops/azuredevops/workitem_create"
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

func TestWorkitemCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// api-version is mandatory on every call; its absence does not fail
		// cleanly against the real service, so assert it here.
		if r.URL.Query().Get("api-version") == "" {
			t.Error("api-version is missing from the request")
		}
		w.Header().Set("Content-Type", "application/json")
		// The type rides in the path behind a LITERAL $ — %24 would 404.
		if !strings.HasSuffix(r.URL.Path, "/_apis/wit/workitems/$Bug") {
			t.Errorf("path = %q, want the literal $Bug (unescaped dollar)", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json-patch+json" {
			t.Errorf("Content-Type = %q, want application/json-patch+json", ct)
		}
		var ops []map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&ops); err != nil {
			t.Fatalf("the body is not a JSON-Patch document: %v", err)
		}
		got := map[string]interface{}{}
		for _, op := range ops {
			if op["op"] != "add" {
				t.Errorf("op = %v, want add (replace fails on an empty field)", op["op"])
			}
			got[op["path"].(string)] = op["value"]
		}
		if got["/fields/System.Title"] != "Checkout is broken" {
			t.Errorf("title was not translated to System.Title: %v", got)
		}
		if got["/fields/System.AssignedTo"] != "jane@contoso.com" {
			t.Errorf(`"assigned to" was not translated to System.AssignedTo: %v`, got)
		}
		if got["/fields/Microsoft.VSTS.Common.Priority"] != float64(1) {
			t.Errorf("priority was not translated: %v", got)
		}
		_, _ = w.Write([]byte(`{"id":42,"url":"https://dev.azure.com/o/_apis/wit/workItems/42"}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "work_item_type", Type: core.ConnectionTypeString, Value: "Bug"},
		&core.Connection{Name: "title", Type: core.ConnectionTypeString, Value: "Checkout is broken"},
		&core.Connection{Name: "fields", Type: core.ConnectionTypeObject, Value: `{"assigned to": "jane@contoso.com", "priority": 1}`},
	))
	if err != nil {
		t.Fatalf("Execute returned a hard error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, error = %v", out["success"], out["error"])
	}
	if out["id"] != "42" {
		t.Errorf("id = %v, want 42", out["id"])
	}
	if out["url"] == "" {
		t.Error("url was not lifted")
	}
}

// TestWorkitemCreateAPIError pins the soft-failure contract: a provider error is data on
// the error port, never a hard Go error that would abort the flow run.
func TestWorkitemCreateAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"TF200016: The following project does not exist: MyProject."}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "work_item_type", Type: core.ConnectionTypeString, Value: "Bug"},
		&core.Connection{Name: "title", Type: core.ConnectionTypeString, Value: "Checkout is broken"},
		&core.Connection{Name: "fields", Type: core.ConnectionTypeObject, Value: `{"assigned to": "jane@contoso.com", "priority": 1}`},
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

// TestWorkitemCreateSignInPage pins the 203 trap: a bad or expired PAT answers 203 with an
// HTML sign-in page rather than 401, so a plain 2xx check would call this a
// success and then fail on an HTML-to-JSON decode.
func TestWorkitemCreateSignInPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNonAuthoritativeInfo)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign In</body></html>"))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "work_item_type", Type: core.ConnectionTypeString, Value: "Bug"},
		&core.Connection{Name: "title", Type: core.ConnectionTypeString, Value: "Checkout is broken"},
		&core.Connection{Name: "fields", Type: core.ConnectionTypeObject, Value: `{"assigned to": "jane@contoso.com", "priority": 1}`},
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
