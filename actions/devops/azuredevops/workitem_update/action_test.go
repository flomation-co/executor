package devops_azuredevops_workitem_update_test

import (
	"encoding/json"
	core "flomation.app/automate/executor"
	action "flomation.app/automate/executor/actions/devops/azuredevops/workitem_update"
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

func TestWorkitemUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// api-version is mandatory on every call; its absence does not fail
		// cleanly against the real service, so assert it here.
		if r.URL.Query().Get("api-version") == "" {
			t.Error("api-version is missing from the request")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json-patch+json" {
			t.Errorf("Content-Type = %q", ct)
		}
		var ops []map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&ops); err != nil {
			t.Fatalf("decode: %v", err)
		}
		byPath := map[string]map[string]interface{}{}
		for _, op := range ops {
			byPath[op["path"].(string)] = op
		}
		if byPath["/fields/System.State"]["op"] != "add" {
			t.Errorf("state op = %v, want add", byPath["/fields/System.State"])
		}
		// null must clear the field, i.e. become a remove.
		if byPath["/fields/System.AssignedTo"]["op"] != "remove" {
			t.Errorf("a null value must become a remove: %v", byPath["/fields/System.AssignedTo"])
		}
		_, _ = w.Write([]byte(`{"id":42,"url":"u"}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "work_item_id", Type: core.ConnectionTypeInteger, Value: 42},
		&core.Connection{Name: "fields", Type: core.ConnectionTypeObject, Value: `{"state": "Active", "assigned to": null}`},
	))
	if err != nil {
		t.Fatalf("Execute returned a hard error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, error = %v", out["success"], out["error"])
	}
	if out["success"] != true {
		t.Errorf("out = %v", out)
	}
}

// TestWorkitemUpdateAPIError pins the soft-failure contract: a provider error is data on
// the error port, never a hard Go error that would abort the flow run.
func TestWorkitemUpdateAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"TF200016: The following project does not exist: MyProject."}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "work_item_id", Type: core.ConnectionTypeInteger, Value: 42},
		&core.Connection{Name: "fields", Type: core.ConnectionTypeObject, Value: `{"state": "Active", "assigned to": null}`},
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

// TestWorkitemUpdateSignInPage pins the 203 trap: a bad or expired PAT answers 203 with an
// HTML sign-in page rather than 401, so a plain 2xx check would call this a
// success and then fail on an HTML-to-JSON decode.
func TestWorkitemUpdateSignInPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNonAuthoritativeInfo)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign In</body></html>"))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "work_item_id", Type: core.ConnectionTypeInteger, Value: 42},
		&core.Connection{Name: "fields", Type: core.ConnectionTypeObject, Value: `{"state": "Active", "assigned to": null}`},
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
