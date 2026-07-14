// Cross-action tests for the six AWX project actions. They live in one file
// (rather than one per action package) because they share the AWX test server and
// the input fixtures, and because the traps worth pinning are traps of the FAMILY:
//
//   - a MANUAL project cannot be synced, and must say so in English rather than
//     surfacing AWX's bare 405;
//   - "Manual" as a LIST FILTER means scm_type= (empty), which is not the same as
//     "Any" (no filter at all) — the one place the design's dropdown values are
//     genuinely ambiguous;
//   - a destructive action refuses without its confirm tick, and does so BEFORE
//     touching AWX;
//   - a 409 with active_jobs is retryable, not a generic failure;
//   - and everything except a bad credential is a SOFT failure — a non-nil error
//     would abort the whole flow.
package infrastructure_awx_project_sync

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"

	create "flomation.app/automate/executor/actions/infrastructure/awx/project_create"
	del "flomation.app/automate/executor/actions/infrastructure/awx/project_delete"
	get "flomation.app/automate/executor/actions/infrastructure/awx/project_get"
	list "flomation.app/automate/executor/actions/infrastructure/awx/project_list"
	update "flomation.app/automate/executor/actions/infrastructure/awx/project_update"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// awxServer answers the API-root discovery probe exactly as a real upstream AWX
// 24.6.1 does, and delegates everything else to h.
func awxServer(h http.HandlerFunc) *httptest.Server {
	awx.ResetAPIRootCacheForTest()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			_, _ = w.Write([]byte(`{"current_version":"/api/v2/","available_versions":{"v2":"/api/v2/"}}`))
		case "/api/v2/me/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"username":"admin"}]}`))
		default:
			h(w, r)
		}
	}))
}

// authInputs is the credential block every action needs, as the flow engine would
// hand it over.
func authInputs(base string) []*core.Connection {
	return []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: base},
		{Name: "auth_method", Type: core.ConnectionTypeString, Value: "token"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func boolean(name string, value bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: value}
}

func integer(name string, value int) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: value}
}

func with(base []*core.Connection, extra ...*core.Connection) []*core.Connection {
	return append(append([]*core.Connection{}, base...), extra...)
}

// mustSucceed asserts the SOFT-failure contract's happy half: a nil error and
// success == true.
func mustSucceed(t *testing.T, out map[string]interface{}, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("hard error (this would abort the whole flow): %v", err)
	}
	if out["success"] != true {
		t.Fatalf("expected success, got success=%v error=%v", out["success"], out["error"])
	}
}

// mustSoftFail asserts the other half: a FAILED result carried on a NIL error, so
// the flow keeps walking and an AI tool loop can read the message and recover.
func mustSoftFail(t *testing.T, out map[string]interface{}, err error, contains string) {
	t.Helper()
	if err != nil {
		t.Fatalf("failure must be SOFT (nil error), got a hard error: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("expected a failure, got success=%v", out["success"])
	}
	msg, _ := out["error"].(string)
	if !strings.Contains(msg, contains) {
		t.Fatalf("error %q does not mention %q", msg, contains)
	}
}

func decodeBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	raw, _ := io.ReadAll(r.Body)
	body := map[string]interface{}{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("request body is not JSON: %v (%s)", err, raw)
		}
	}
	return body
}

// ---------------------------------------------------------------------------
// project_sync
// ---------------------------------------------------------------------------

func TestProjectSyncStartsASyncAndReportsTheUpdateID(t *testing.T) {
	posted := false
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/7/update/":
			_, _ = w.Write([]byte(`{"can_update":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/projects/7/update/":
			posted = true
			// 202 ACCEPTED, not 201 — and the id appears at BOTH keys.
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"project_update":42,"id":42,"status":"pending"}`))
		default:
			t.Errorf("unexpected call %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(authInputs(srv.URL), str("project_id", "7")))
	mustSucceed(t, out, err)

	if !posted {
		t.Fatal("no POST to projects/7/update/ was made")
	}
	if out["project_update_id"] != "42" {
		t.Fatalf("project_update_id = %v, want 42", out["project_update_id"])
	}
	if out["finished"] != false {
		t.Fatalf("finished = %v — a fire-and-forget sync has not finished", out["finished"])
	}
}

// ★ THE FAMILY TRAP. can_update is bool(scm_type), so a MANUAL project can never
// be synced and AWX answers the POST with 405 METHOD NOT ALLOWED — not a 400, and
// with nothing in it to explain why. The action must pre-flight, never POST, and
// say what is actually wrong.
func TestProjectSyncRefusesAManualProjectWithAPlainEnglishReason(t *testing.T) {
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/7/update/":
			_, _ = w.Write([]byte(`{"can_update":false}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/7/":
			_, _ = w.Write([]byte(`{"id":7,"name":"Local Playbooks","scm_type":""}`))
		case r.Method == http.MethodPost:
			t.Error("the sync was POSTed anyway — AWX would answer 405 and the operator would be none the wiser")
			w.WriteHeader(http.StatusMethodNotAllowed)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(authInputs(srv.URL), str("project_id", "7")))
	mustSoftFail(t, out, err, "MANUAL project")
}

func TestProjectSyncWaitsForTheUpdateToFinish(t *testing.T) {
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/7/update/":
			_, _ = w.Write([]byte(`{"can_update":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/projects/7/update/":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"project_update":42,"id":42,"status":"pending"}`))
		// WaitForJob polls the cheap LIST endpoint, not the detail view.
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project_updates/":
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":42,"status":"successful","finished":"2026-07-14T09:00:05Z","failed":false}]}`))
		// …then takes exactly one detail GET for the terminal record.
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project_updates/42/":
			_, _ = w.Write([]byte(`{"id":42,"status":"successful","finished":"2026-07-14T09:00:05Z","failed":false,"elapsed":4.2,"scm_revision":"9f1c0d2","event_processing_finished":true}`))
		default:
			t.Errorf("unexpected call %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(authInputs(srv.URL),
		str("project_id", "7"),
		boolean("wait_for_completion", true),
		integer("poll_interval_seconds", 1),
		integer("timeout_seconds", 30),
	))
	mustSucceed(t, out, err)

	if out["status"] != "successful" || out["finished"] != true || out["failed"] != false {
		t.Fatalf("status=%v finished=%v failed=%v", out["status"], out["finished"], out["failed"])
	}
	if out["scm_revision"] != "9f1c0d2" {
		t.Fatalf("scm_revision = %v, want 9f1c0d2", out["scm_revision"])
	}
	if out["elapsed"] != "4.2" {
		t.Fatalf("elapsed = %v, want 4.2", out["elapsed"])
	}
	if out["timed_out"] != false {
		t.Fatalf("timed_out = %v", out["timed_out"])
	}
}

// A failed sync is a SOFT failure carrying the outputs, so a downstream node can
// read the status — and Ignore Sync Failure flips it back to a success.
func TestProjectSyncReportsAFailedSyncUnlessIgnored(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/7/update/":
			_, _ = w.Write([]byte(`{"can_update":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/projects/7/update/":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"project_update":42,"id":42}`))
		case r.URL.Path == "/api/v2/project_updates/":
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":42,"status":"failed","finished":"2026-07-14T09:00:05Z","failed":true}]}`))
		case r.URL.Path == "/api/v2/project_updates/42/":
			_, _ = w.Write([]byte(`{"id":42,"status":"failed","finished":"2026-07-14T09:00:05Z","failed":true,"job_explanation":"","event_processing_finished":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}

	srv := awxServer(handler)
	out, err := Execute(nil, nil, with(authInputs(srv.URL),
		str("project_id", "7"), boolean("wait_for_completion", true), integer("poll_interval_seconds", 1)))
	srv.Close()
	mustSoftFail(t, out, err, `ended "failed"`)
	if out["failed"] != true {
		t.Fatalf("failed = %v — the outputs must still be populated on a soft failure", out["failed"])
	}

	srv = awxServer(handler)
	defer srv.Close()
	out, err = Execute(nil, nil, with(authInputs(srv.URL),
		str("project_id", "7"), boolean("wait_for_completion", true), integer("poll_interval_seconds", 1),
		boolean("ignore_job_failure", true)))
	mustSucceed(t, out, err)
	if out["failed"] != true {
		t.Fatalf("failed = %v — Ignore Sync Failure must not rewrite what AWX reported", out["failed"])
	}
}

// A missing credential is the ONE hard error in the whole node: it aborts the flow
// rather than letting it walk on with a mis-configured node.
func TestMissingCredentialIsAHardError(t *testing.T) {
	out, err := Execute(nil, nil, []*core.Connection{str("awx_url", "https://awx.example.com"), str("project_id", "7")})
	if err == nil {
		t.Fatal("a missing API token must be a HARD error")
	}
	if out != nil {
		t.Fatalf("a hard error must return nil outputs, got %v", out)
	}
}

// ---------------------------------------------------------------------------
// project_list
// ---------------------------------------------------------------------------

func TestProjectListFiltersAndPages(t *testing.T) {
	var query string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/projects/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"count":3,"next":"/api/v2/projects/?page=2","results":[{"id":7,"name":"Acme Playbooks","status":"successful"}]}`))
	})
	defer srv.Close()

	out, err := list.Execute(nil, nil, with(authInputs(srv.URL),
		str("search", "acme"), str("organization_id", "1"), str("scm_type", "git"), str("status", "successful")))
	mustSucceed(t, out, err)

	for _, want := range []string{"search=acme", "organization=1", "scm_type=git", "status=successful", "order_by=name", "page_size=50"} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q is missing %q", query, want)
		}
	}
	if out["count"] != 1 || out["total_count"] != 3 || out["has_more"] != true {
		t.Fatalf("count=%v total_count=%v has_more=%v", out["count"], out["total_count"], out["has_more"])
	}
}

// ★ AWX stores a MANUAL project's scm_type as the EMPTY STRING, so filtering for
// manual projects means sending scm_type= — an empty value that every "skip if
// blank" helper drops, and which is indistinguishable from the untouched
// dropdown. Hence the "manual" sentinel: "Any" must send no filter at all, and
// "Manual" must send an empty one.
func TestProjectListManualFilterSendsAnEmptySCMType(t *testing.T) {
	var query string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
	})
	defer srv.Close()

	out, err := list.Execute(nil, nil, with(authInputs(srv.URL), str("scm_type", "manual")))
	mustSucceed(t, out, err)
	if !strings.Contains(query, "scm_type=") {
		t.Fatalf("query %q must carry an empty scm_type — that is what selects manual projects", query)
	}
	if strings.Contains(query, "scm_type=manual") {
		t.Fatalf("query %q leaked the dropdown sentinel to AWX; AWX has no such scm_type", query)
	}

	// And "Any" (the blank option) must not filter at all.
	srv2 := awxServer(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
	})
	defer srv2.Close()

	out, err = list.Execute(nil, nil, with(authInputs(srv2.URL), str("scm_type", "")))
	mustSucceed(t, out, err)
	if strings.Contains(query, "scm_type") {
		t.Fatalf("query %q must not filter by scm_type when the operator chose Any", query)
	}
}

// ---------------------------------------------------------------------------
// project_get
// ---------------------------------------------------------------------------

func TestProjectGetReturnsTheProjectAndItsPlaybooks(t *testing.T) {
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/projects/7/":
			_, _ = w.Write([]byte(`{"id":7,"name":"Acme Playbooks","status":"successful","scm_revision":"9f1c0d2","last_updated":"2026-07-14T09:00:05Z"}`))
		case "/api/v2/projects/7/playbooks/":
			// A bare JSON ARRAY, not AWX's usual paginated envelope.
			_, _ = w.Write([]byte(`["site.yml","deploy.yml"]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	out, err := get.Execute(nil, nil, with(authInputs(srv.URL), str("project_id", "7")))
	mustSucceed(t, out, err)

	if out["id"] != "7" || out["status"] != "successful" || out["scm_revision"] != "9f1c0d2" {
		t.Fatalf("id=%v status=%v scm_revision=%v", out["id"], out["status"], out["scm_revision"])
	}
	files, _ := out["playbook_files"].([]interface{})
	if len(files) != 2 || files[0] != "site.yml" {
		t.Fatalf("playbook_files = %v", out["playbook_files"])
	}
}

// A project that has never been synced has no checkout on disk, and AWX answers
// its playbooks endpoint with a 404. That must not fail the get.
func TestProjectGetSurvivesAMissingPlaybookList(t *testing.T) {
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/projects/7/" {
			_, _ = w.Write([]byte(`{"id":7,"name":"Never Synced","status":"never updated"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	out, err := get.Execute(nil, nil, with(authInputs(srv.URL), str("project_id", "7")))
	mustSucceed(t, out, err)
	if files, _ := out["playbook_files"].([]interface{}); len(files) != 0 {
		t.Fatalf("playbook_files = %v, want an empty list", out["playbook_files"])
	}
}

// AWX hides objects you cannot SEE behind a 404 rather than a 403, so a 404 is a
// soft failure that must never be reported as "deleted".
func TestProjectGetSoftFailsOnAMissingProject(t *testing.T) {
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not found."}`))
	})
	defer srv.Close()

	out, err := get.Execute(nil, nil, with(authInputs(srv.URL), str("project_id", "999")))
	mustSoftFail(t, out, err, "404")
}

// ---------------------------------------------------------------------------
// project_create
// ---------------------------------------------------------------------------

func TestProjectCreatePostsTheProject(t *testing.T) {
	var body map[string]interface{}
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/projects/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body = decodeBody(t, r)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":7,"name":"Acme Playbooks"}`))
	})
	defer srv.Close()

	out, err := create.Execute(nil, nil, with(authInputs(srv.URL),
		str("name", "Acme Playbooks"),
		str("organization_id", "1"),
		str("scm_type", "git"),
		str("scm_url", "https://github.com/acme/playbooks.git"),
		str("scm_branch", "main"),
		str("credential_id", "3"),
		boolean("scm_update_on_launch", true),
		integer("sync_timeout", 120),
	))
	mustSucceed(t, out, err)

	if out["id"] != "7" {
		t.Fatalf("id = %v, want 7", out["id"])
	}
	// AWX wants real JSON integers for its id fields, not the strings a live
	// dropdown writes.
	if body["organization"] != float64(1) || body["credential"] != float64(3) {
		t.Fatalf("organization=%#v credential=%#v — both must be JSON numbers", body["organization"], body["credential"])
	}
	if body["scm_type"] != "git" || body["scm_url"] != "https://github.com/acme/playbooks.git" {
		t.Fatalf("body = %#v", body)
	}
	if body["scm_update_on_launch"] != true {
		t.Fatalf("scm_update_on_launch = %#v", body["scm_update_on_launch"])
	}
	// sync_timeout is AWX's plain "timeout" on the project model.
	if body["timeout"] != float64(120) {
		t.Fatalf("timeout = %#v, want 120", body["timeout"])
	}
	// An untouched checkbox is OMITTED, not sent as false: with scm_type="" AWX
	// refuses any of these set at all, so the tri-state matters.
	if _, present := body["scm_clean"]; present {
		t.Fatalf("scm_clean was sent even though the operator never touched it: %#v", body)
	}
}

func TestProjectCreateRequiresAnOrganizationAndAnSCMURL(t *testing.T) {
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("AWX must not be called at all: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()

	// Organization: DRF calls it optional, but ProjectAccess.can_add makes it a
	// 403 PermissionDenied for a non-superuser — an unexplainable error we refuse
	// to let the operator hit.
	out, err := create.Execute(nil, nil, with(authInputs(srv.URL), str("name", "Acme")))
	mustSoftFail(t, out, err, "Organization is required")

	// scm_url is mandatory once a source-control type is chosen.
	out, err = create.Execute(nil, nil, with(authInputs(srv.URL),
		str("name", "Acme"), str("organization_id", "1"), str("scm_type", "git")))
	mustSoftFail(t, out, err, "Source Control URL is required")
}

// ---------------------------------------------------------------------------
// project_update
// ---------------------------------------------------------------------------

func TestProjectUpdatePatchesOnlyTheFieldsGiven(t *testing.T) {
	var method string
	var body map[string]interface{}
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/projects/7/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		method = r.Method
		body = decodeBody(t, r)
		_, _ = w.Write([]byte(`{"id":7,"name":"Acme Playbooks","scm_branch":"release"}`))
	})
	defer srv.Close()

	out, err := update.Execute(nil, nil, with(authInputs(srv.URL),
		str("project_id", "7"),
		str("scm_branch", "release"),
		boolean("confirm_destructive", true),
	))
	mustSucceed(t, out, err)

	// ★ PATCH, NEVER PUT: AWX copies every model default onto the serializer, so a
	// PUT that omits a field RESETS it.
	if method != http.MethodPatch {
		t.Fatalf("method = %s, want PATCH — a PUT would reset every field the operator left blank", method)
	}
	if len(body) != 1 || body["scm_branch"] != "release" {
		t.Fatalf("body = %#v — only the fields the operator filled in may be sent", body)
	}
}

func TestProjectUpdateRefusesWithoutTheDestructiveConfirmation(t *testing.T) {
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("AWX must not be called: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()

	out, err := update.Execute(nil, nil, with(authInputs(srv.URL), str("project_id", "7"), str("name", "Renamed")))
	mustSoftFail(t, out, err, "Confirm Destructive Action")
}

func TestProjectUpdateRefusesAnEmptyChange(t *testing.T) {
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("AWX must not be called: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()

	out, err := update.Execute(nil, nil, with(authInputs(srv.URL),
		str("project_id", "7"), boolean("confirm_destructive", true)))
	mustSoftFail(t, out, err, "Nothing to change")
}

// ---------------------------------------------------------------------------
// project_delete
// ---------------------------------------------------------------------------

func TestProjectDeleteDeletes(t *testing.T) {
	deleted := false
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/api/v2/projects/7/" {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	out, err := del.Execute(nil, nil, with(authInputs(srv.URL),
		str("project_id", "7"), boolean("confirm_destructive", true)))
	mustSucceed(t, out, err)

	if !deleted {
		t.Fatal("no DELETE was issued")
	}
	if out["deleted"] != true || out["id"] != "7" {
		t.Fatalf("deleted=%v id=%v", out["deleted"], out["id"])
	}
}

func TestProjectDeleteRefusesWithoutTheDestructiveConfirmation(t *testing.T) {
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("AWX must not be called: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()

	out, err := del.Execute(nil, nil, with(authInputs(srv.URL), str("project_id", "7")))
	mustSoftFail(t, out, err, "Confirm Destructive Action")
}

// AWX refuses to delete a project a job is still running against, with a 409 and
// an active_jobs envelope. That is RETRYABLE — "wait and try again" — not a
// generic failure, and the message has to say so.
func TestProjectDeleteReportsActiveJobsAsRetryable(t *testing.T) {
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"Resource is being used by running jobs.","active_jobs":[{"type":"project_update","id":42}]}`))
	})
	defer srv.Close()

	out, err := del.Execute(nil, nil, with(authInputs(srv.URL),
		str("project_id", "7"), boolean("confirm_destructive", true)))
	mustSoftFail(t, out, err, "still running")

	msg, _ := out["error"].(string)
	if !strings.Contains(msg, "project update 42") {
		t.Fatalf("error %q must name the job that is holding the project", msg)
	}
}
