package devops_azuredevops_pr_comment_add_test

import (
	"encoding/json"
	core "flomation.app/automate/executor"
	action "flomation.app/automate/executor/actions/devops/azuredevops/pr_comment_add"
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

func TestPrCommentAdd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// api-version is mandatory on every call; its absence does not fail
		// cleanly against the real service, so assert it here.
		if r.URL.Query().Get("api-version") == "" {
			t.Error("api-version is missing from the request")
		}
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(r.URL.Path, "/pullRequests/128/threads") {
			t.Errorf("path = %q, want a new thread (a top-level comment IS a thread)", r.URL.Path)
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		comments := body["comments"].([]interface{})
		first := comments[0].(map[string]interface{})
		if first["content"] != "LGTM" {
			t.Errorf("comments = %v", body["comments"])
		}
		if body["status"] != "active" {
			t.Errorf("status = %v, want the active default", body["status"])
		}
		_, _ = w.Write([]byte(`{"id":9,"comments":[{"id":1,"content":"LGTM"}]}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "repository", Type: core.ConnectionTypeString, Value: "MyRepo"},
		&core.Connection{Name: "pull_request_id", Type: core.ConnectionTypeInteger, Value: 128},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: `LGTM`},
	))
	if err != nil {
		t.Fatalf("Execute returned a hard error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, error = %v", out["success"], out["error"])
	}
	if out["id"] != "9" {
		t.Errorf("id = %v, want the thread id", out["id"])
	}
}

// TestPrCommentAddAPIError pins the soft-failure contract: a provider error is data on
// the error port, never a hard Go error that would abort the flow run.
func TestPrCommentAddAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"TF200016: The following project does not exist: MyProject."}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "repository", Type: core.ConnectionTypeString, Value: "MyRepo"},
		&core.Connection{Name: "pull_request_id", Type: core.ConnectionTypeInteger, Value: 128},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: `LGTM`},
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

// TestPrCommentAddSignInPage pins the 203 trap: a bad or expired PAT answers 203 with an
// HTML sign-in page rather than 401, so a plain 2xx check would call this a
// success and then fail on an HTML-to-JSON decode.
func TestPrCommentAddSignInPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNonAuthoritativeInfo)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign In</body></html>"))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "repository", Type: core.ConnectionTypeString, Value: "MyRepo"},
		&core.Connection{Name: "pull_request_id", Type: core.ConnectionTypeInteger, Value: 128},
		&core.Connection{Name: "content", Type: core.ConnectionTypeText, Value: `LGTM`},
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
