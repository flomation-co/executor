package devops_azuredevops_pr_create_test

import (
	"encoding/json"
	core "flomation.app/automate/executor"
	action "flomation.app/automate/executor/actions/devops/azuredevops/pr_create"
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

func TestPrCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// api-version is mandatory on every call; its absence does not fail
		// cleanly against the real service, so assert it here.
		if r.URL.Query().Get("api-version") == "" {
			t.Error("api-version is missing from the request")
		}
		w.Header().Set("Content-Type", "application/json")
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["sourceRefName"] != "refs/heads/feature/x" || body["targetRefName"] != "refs/heads/main" {
			t.Errorf("refs = %v / %v, want full refs — a bare branch name silently 400s",
				body["sourceRefName"], body["targetRefName"])
		}
		if len(body["reviewers"].([]interface{})) != 2 {
			t.Errorf("reviewers = %v", body["reviewers"])
		}
		if len(body["workItemRefs"].([]interface{})) != 1 {
			t.Errorf("workItemRefs = %v", body["workItemRefs"])
		}
		_, _ = w.Write([]byte(`{"pullRequestId":128,"title":"Fix checkout","url":"u"}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "repository", Type: core.ConnectionTypeString, Value: "MyRepo"},
		&core.Connection{Name: "source_branch", Type: core.ConnectionTypeString, Value: "feature/x"},
		&core.Connection{Name: "target_branch", Type: core.ConnectionTypeString, Value: "main"},
		&core.Connection{Name: "title", Type: core.ConnectionTypeString, Value: "Fix checkout"},
		&core.Connection{Name: "description", Type: core.ConnectionTypeText, Value: `why`},
		&core.Connection{Name: "reviewers", Type: core.ConnectionTypeString, Value: "id-1,id-2"},
		&core.Connection{Name: "work_item_ids", Type: core.ConnectionTypeString, Value: "42"},
	))
	if err != nil {
		t.Fatalf("Execute returned a hard error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, error = %v", out["success"], out["error"])
	}
	// A PR's identity is pullRequestId; there is no generic "id" field to lift.
	if out["id"] != "128" {
		t.Errorf("id = %v, want 128 from pullRequestId", out["id"])
	}
}

// TestPrCreateAPIError pins the soft-failure contract: a provider error is data on
// the error port, never a hard Go error that would abort the flow run.
func TestPrCreateAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"TF200016: The following project does not exist: MyProject."}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "repository", Type: core.ConnectionTypeString, Value: "MyRepo"},
		&core.Connection{Name: "source_branch", Type: core.ConnectionTypeString, Value: "feature/x"},
		&core.Connection{Name: "target_branch", Type: core.ConnectionTypeString, Value: "main"},
		&core.Connection{Name: "title", Type: core.ConnectionTypeString, Value: "Fix checkout"},
		&core.Connection{Name: "description", Type: core.ConnectionTypeText, Value: `why`},
		&core.Connection{Name: "reviewers", Type: core.ConnectionTypeString, Value: "id-1,id-2"},
		&core.Connection{Name: "work_item_ids", Type: core.ConnectionTypeString, Value: "42"},
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

// TestPrCreateSignInPage pins the 203 trap: a bad or expired PAT answers 203 with an
// HTML sign-in page rather than 401, so a plain 2xx check would call this a
// success and then fail on an HTML-to-JSON decode.
func TestPrCreateSignInPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNonAuthoritativeInfo)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign In</body></html>"))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "repository", Type: core.ConnectionTypeString, Value: "MyRepo"},
		&core.Connection{Name: "source_branch", Type: core.ConnectionTypeString, Value: "feature/x"},
		&core.Connection{Name: "target_branch", Type: core.ConnectionTypeString, Value: "main"},
		&core.Connection{Name: "title", Type: core.ConnectionTypeString, Value: "Fix checkout"},
		&core.Connection{Name: "description", Type: core.ConnectionTypeText, Value: `why`},
		&core.Connection{Name: "reviewers", Type: core.ConnectionTypeString, Value: "id-1,id-2"},
		&core.Connection{Name: "work_item_ids", Type: core.ConnectionTypeString, Value: "42"},
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
