package devops_azuredevops_pipeline_run_get_all_test

import (
	core "flomation.app/automate/executor"
	action "flomation.app/automate/executor/actions/devops/azuredevops/pipeline_run_get_all"
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

func TestPipelineRunGetAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// api-version is mandatory on every call; its absence does not fail
		// cleanly against the real service, so assert it here.
		if r.URL.Query().Get("api-version") == "" {
			t.Error("api-version is missing from the request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":3,"value":[{"id":3},{"id":2},{"id":1}]}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "pipeline_id", Type: core.ConnectionTypeInteger, Value: 12},
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: 2},
	))
	if err != nil {
		t.Fatalf("Execute returned a hard error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, error = %v", out["success"], out["error"])
	}
	// The endpoint honours no paging parameter, so Limit must trim our side.
	if out["count"] != 2 {
		t.Errorf("count = %v, want 2 — Limit must trim a list the server would not", out["count"])
	}
}

// TestPipelineRunGetAllAPIError pins the soft-failure contract: a provider error is data on
// the error port, never a hard Go error that would abort the flow run.
func TestPipelineRunGetAllAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"TF200016: The following project does not exist: MyProject."}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "pipeline_id", Type: core.ConnectionTypeInteger, Value: 12},
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: 2},
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

// TestPipelineRunGetAllSignInPage pins the 203 trap: a bad or expired PAT answers 203 with an
// HTML sign-in page rather than 401, so a plain 2xx check would call this a
// success and then fail on an HTML-to-JSON decode.
func TestPipelineRunGetAllSignInPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNonAuthoritativeInfo)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign In</body></html>"))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "pipeline_id", Type: core.ConnectionTypeInteger, Value: 12},
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: 2},
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
