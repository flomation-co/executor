package devops_azuredevops_pipeline_run_test

import (
	"encoding/json"
	core "flomation.app/automate/executor"
	action "flomation.app/automate/executor/actions/devops/azuredevops/pipeline_run"
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

func TestPipelineRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// api-version is mandatory on every call; its absence does not fail
		// cleanly against the real service, so assert it here.
		if r.URL.Query().Get("api-version") == "" {
			t.Error("api-version is missing from the request")
		}
		w.Header().Set("Content-Type", "application/json")
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// A bare branch name must have been expanded to a full ref — Azure
		// DevOps silently 400s on "main".
		res := body["resources"].(map[string]interface{})
		repos := res["repositories"].(map[string]interface{})
		self := repos["self"].(map[string]interface{})
		if self["refName"] != "refs/heads/main" {
			t.Errorf("refName = %v, want refs/heads/main", self["refName"])
		}
		// A scalar variable must have been wrapped in the {value: …} envelope.
		vars := body["variables"].(map[string]interface{})
		tag := vars["releaseTag"].(map[string]interface{})
		if tag["value"] != "v1.2.3" {
			t.Errorf("releaseTag = %v, want the {value:…} envelope", vars["releaseTag"])
		}
		if params := body["templateParameters"].(map[string]interface{}); params["environment"] != "staging" {
			t.Errorf("templateParameters = %v", body["templateParameters"])
		}
		if skip := body["stagesToSkip"].([]interface{}); len(skip) != 2 || skip[0] != "Notify" || skip[1] != "Cleanup" {
			t.Errorf("stagesToSkip = %v, want [Notify Cleanup]", body["stagesToSkip"])
		}
		_, _ = w.Write([]byte(`{"id":99,"state":"inProgress","_links":{"web":{"href":"https://dev.azure.com/o/_build/results?buildId=99"}}}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "pipeline_id", Type: core.ConnectionTypeInteger, Value: 12},
		&core.Connection{Name: "branch", Type: core.ConnectionTypeString, Value: "main"},
		&core.Connection{Name: "variables", Type: core.ConnectionTypeObject, Value: `{"releaseTag": "v1.2.3"}`},
		&core.Connection{Name: "template_parameters", Type: core.ConnectionTypeObject, Value: `{"environment": "staging"}`},
		&core.Connection{Name: "stages_to_skip", Type: core.ConnectionTypeString, Value: "Notify, Cleanup"},
	))
	if err != nil {
		t.Fatalf("Execute returned a hard error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, error = %v", out["success"], out["error"])
	}
	if out["id"] != "99" {
		t.Errorf("id = %v, want 99", out["id"])
	}
	if out["run_state"] != "inProgress" {
		t.Errorf("run_state = %v", out["run_state"])
	}
	if out["run_url"] == "" {
		t.Error("run_url was not lifted from _links.web.href")
	}
}

// TestPipelineRunAPIError pins the soft-failure contract: a provider error is data on
// the error port, never a hard Go error that would abort the flow run.
func TestPipelineRunAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"TF200016: The following project does not exist: MyProject."}`))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "pipeline_id", Type: core.ConnectionTypeInteger, Value: 12},
		&core.Connection{Name: "branch", Type: core.ConnectionTypeString, Value: "main"},
		&core.Connection{Name: "variables", Type: core.ConnectionTypeObject, Value: `{"releaseTag": "v1.2.3"}`},
		&core.Connection{Name: "template_parameters", Type: core.ConnectionTypeObject, Value: `{"environment": "staging"}`},
		&core.Connection{Name: "stages_to_skip", Type: core.ConnectionTypeString, Value: "Notify, Cleanup"},
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

// TestPipelineRunSignInPage pins the 203 trap: a bad or expired PAT answers 203 with an
// HTML sign-in page rather than 401, so a plain 2xx check would call this a
// success and then fail on an HTML-to-JSON decode.
func TestPipelineRunSignInPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNonAuthoritativeInfo)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign In</body></html>"))
	}))
	defer srv.Close()

	out, err := action.Execute(nil, nil, conns(srv.URL,
		&core.Connection{Name: "project", Type: core.ConnectionTypeString, Value: "MyProject"},
		&core.Connection{Name: "pipeline_id", Type: core.ConnectionTypeInteger, Value: 12},
		&core.Connection{Name: "branch", Type: core.ConnectionTypeString, Value: "main"},
		&core.Connection{Name: "variables", Type: core.ConnectionTypeObject, Value: `{"releaseTag": "v1.2.3"}`},
		&core.Connection{Name: "template_parameters", Type: core.ConnectionTypeObject, Value: `{"environment": "staging"}`},
		&core.Connection{Name: "stages_to_skip", Type: core.ConnectionTypeString, Value: "Notify, Cleanup"},
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
