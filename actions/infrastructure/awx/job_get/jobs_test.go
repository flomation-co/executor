// Tests for the six job-family AWX actions. They live together because they share
// one AWX fixture server and because the traps they guard against (the 405 on a
// finished cancel, the reserved `limit` param, the "too large" sentence AWX serves
// with a 200) are properties of the family, not of one action.
package infrastructure_awx_job_get

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	cancel "flomation.app/automate/executor/actions/infrastructure/awx/job_cancel"
	events "flomation.app/automate/executor/actions/infrastructure/awx/job_events_list"
	list "flomation.app/automate/executor/actions/infrastructure/awx/job_list"
	relaunch "flomation.app/automate/executor/actions/infrastructure/awx/job_relaunch"
	stdout "flomation.app/automate/executor/actions/infrastructure/awx/job_stdout_get"
	. "github.com/onsi/gomega"
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

// auth is the credential block every test starts from. The token is a realistic
// 30-character PAT, not a toy: awx.Redact scrubs it out of messages with a plain
// substring replace, so a short one would corrupt the very errors we assert on.
func auth(base string) []*core.Connection {
	return []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: base},
		{Name: "auth_method", Type: core.ConnectionTypeString, Value: "token"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}
}

func str(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

func boolean(name string, val bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: val}
}

func integer(name string, val int64) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: val}
}

// ---------------------------------------------------------------------------
// job_get
// ---------------------------------------------------------------------------

func TestGetJobHappyPath(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/jobs/7/"))
		_, _ = w.Write([]byte(`{"id":7,"name":"Deploy web","status":"successful","failed":false,
			"finished":"2026-07-14T10:00:00Z","elapsed":12.5,
			"artifacts":{"build":"42"},"host_status_counts":{"ok":3},
			"event_processing_finished":true,"result_traceback":""}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, append(auth(srv.URL), str("job_id", "7")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("7"))
	Expect(out["job_id"]).To(Equal("7"))
	Expect(out["job_kind"]).To(Equal("job"))
	Expect(out["status"]).To(Equal("successful"))
	Expect(out["finished"]).To(Equal(true))
	Expect(out["elapsed"]).To(Equal("12.5"))
	Expect(out["artifacts"]).To(Equal(map[string]interface{}{"build": "42"}))
	Expect(out["event_processing_finished"]).To(Equal(true))
	Expect(out["job_url"]).To(ContainSubstring("/#/jobs/playbook/7/output"))
}

// artifacts is EITHER a JSON object OR the literal no_log sentinel STRING.
// Decoding it into map[string]interface{} would fail on every no_log job.
func TestGetJobArtifactsMayBeTheNoLogSentinelString(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":7,"status":"successful","finished":"2026-07-14T10:00:00Z",
			"artifacts":"$hidden due to Ansible no_log flag$"}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, append(auth(srv.URL), str("job_id", "7")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["artifacts"]).To(Equal("$hidden due to Ansible no_log flag$"))
}

// A workflow job is a pure orchestration record: no artifacts, no host results, no
// events of its own. Emit nulls rather than fabricating empty objects.
func TestGetJobWorkflowEmitsNullsNotEmptyObjects(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/workflow_jobs/9/"))
		_, _ = w.Write([]byte(`{"id":9,"status":"successful","finished":"2026-07-14T10:00:00Z"}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, append(auth(srv.URL),
		str("job_kind", "workflow_job"), str("job_id", "9")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["artifacts"]).To(BeNil())
	Expect(out["host_status_counts"]).To(BeNil())
	Expect(out["event_processing_finished"]).To(BeNil())
	Expect(out["job_url"]).To(ContainSubstring("/#/jobs/workflow/9/output"))
}

// An AWX 404 is a SOFT failure: a non-nil Go error would abort the whole flow.
func TestGetJobNotFoundIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not found."}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, append(auth(srv.URL), str("job_id", "404")))
	Expect(err).To(BeNil()) // ★ SOFT
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("404"))
}

// A missing credential is the ONE hard failure — the node is mis-configured.
func TestMissingCredentialIsAHardFailure(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		str("awx_url", "https://awx.example.com"),
		str("job_id", "7"),
	})
	Expect(out).To(BeNil())
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("API Token"))
}

// ---------------------------------------------------------------------------
// job_stdout_get
// ---------------------------------------------------------------------------

func TestGetJobStdoutUsesTxtDownload(t *testing.T) {
	RegisterTestingT(t)
	var format string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/jobs/7/stdout/"))
		format = r.URL.Query().Get("format")
		_, _ = w.Write([]byte("PLAY [web] ***\nok: [host1]\n"))
	})
	defer srv.Close()

	out, err := stdout.Execute(nil, nil, append(auth(srv.URL), str("job_id", "7")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	// ?format=txt would be capped at 1 MiB and answer the cap with an English
	// sentence; txt_download is uncapped and streams from disk.
	Expect(format).To(Equal("txt_download"))
	Expect(out["stdout"]).To(ContainSubstring("ok: [host1]"))
	Expect(out["byte_count"]).To(Equal(27))
	Expect(out["truncated"]).To(Equal(false))
}

// ★ THE TRAP: over the display cap AWX answers HTTP 200 whose BODY IS AN ENGLISH
// SENTENCE. A naive client stores that apology AS the playbook output. It must be
// reported as an error, never as data.
func TestGetJobStdoutNeverStoresTheTooLargeSentenceAsOutput(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Standard Output too large to display (2097152 bytes), only download supported for sizes over 1048576 bytes."))
	})
	defer srv.Close()

	out, err := stdout.Execute(nil, nil, append(auth(srv.URL), str("job_id", "7")))
	Expect(err).To(BeNil()) // soft
	Expect(out["success"]).To(Equal(false))
	Expect(out["stdout"]).To(BeNil())
	Expect(out["error"]).To(ContainSubstring("too large"))
}

func TestGetJobStdoutTruncatesAtMaxBytes(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("0123456789"))
	})
	defer srv.Close()

	out, err := stdout.Execute(nil, nil, append(auth(srv.URL),
		str("job_id", "7"), integer("max_bytes", 4)))
	Expect(err).To(BeNil())
	Expect(out["stdout"]).To(Equal("0123"))
	Expect(out["truncated"]).To(Equal(true))
	Expect(out["byte_count"]).To(Equal(4))
}

func TestGetJobStdoutRefusesAWorkflowJob(t *testing.T) {
	RegisterTestingT(t)
	out, err := stdout.Execute(nil, nil, append(auth("https://awx.example.com"),
		str("job_kind", "workflow_job"), str("job_id", "9")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("no output of its own"))
}

// ---------------------------------------------------------------------------
// job_cancel
// ---------------------------------------------------------------------------

func TestCancelJobHappyPath(t *testing.T) {
	RegisterTestingT(t)
	posted := false
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/jobs/7/cancel/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"can_cancel":true}`))
		case r.URL.Path == "/api/v2/jobs/7/cancel/" && r.Method == http.MethodPost:
			posted = true
			w.WriteHeader(http.StatusAccepted) // 202 with a COMPLETELY EMPTY body
		case r.URL.Path == "/api/v2/jobs/7/":
			_, _ = w.Write([]byte(`{"id":7,"status":"canceled","finished":"2026-07-14T10:00:00Z"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	out, err := cancel.Execute(nil, nil, append(auth(srv.URL),
		str("job_id", "7"), boolean("confirm_destructive", true)))
	Expect(err).To(BeNil())
	Expect(posted).To(BeTrue())
	Expect(out["success"]).To(Equal(true))
	Expect(out["job_id"]).To(Equal("7"))
	Expect(out["status"]).To(Equal("canceled"))
	Expect(out["was_cancellable"]).To(Equal(true))
	Expect(out["already_finished"]).To(Equal(false))
}

// ★ THE TRAP: POSTing /cancel/ on an ALREADY-FINISHED job answers 405 Method Not
// Allowed — not 409, not 400. That is "already terminal", NOT an error.
func TestCancelJobOnAFinishedJobIs405NotAnError(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/jobs/7/cancel/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"can_cancel":false}`))
		case r.URL.Path == "/api/v2/jobs/7/cancel/" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(`{"detail":"Method \"POST\" not allowed."}`))
		case r.URL.Path == "/api/v2/jobs/7/":
			_, _ = w.Write([]byte(`{"id":7,"status":"successful","finished":"2026-07-14T10:00:00Z"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	out, err := cancel.Execute(nil, nil, append(auth(srv.URL),
		str("job_id", "7"), boolean("confirm_destructive", true)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true)) // ★ not a failure
	Expect(out["already_finished"]).To(Equal(true))
	Expect(out["was_cancellable"]).To(Equal(false))
	Expect(out["status"]).To(Equal("successful"))
	Expect(out["error"]).To(Equal(""))
	Expect(out["tool_result"]).To(ContainSubstring("already finished"))
}

// The can_cancel pre-check is inherently racy: it can say true and the POST still
// 405. The 405 handling, not the pre-check, is what is load-bearing.
func TestCancelJobHandlesTheRaceBetweenCanCancelAndThePost(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/jobs/7/cancel/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"can_cancel":true}`)) // it was cancellable a moment ago…
		case r.URL.Path == "/api/v2/jobs/7/cancel/" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusMethodNotAllowed) // …and finished in the race
		case r.URL.Path == "/api/v2/jobs/7/":
			_, _ = w.Write([]byte(`{"id":7,"status":"successful","finished":"2026-07-14T10:00:00Z"}`))
		}
	})
	defer srv.Close()

	out, err := cancel.Execute(nil, nil, append(auth(srv.URL),
		str("job_id", "7"), boolean("confirm_destructive", true)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["was_cancellable"]).To(Equal(true))
	Expect(out["already_finished"]).To(Equal(true))
}

func TestCancelJobRefusesWithoutConfirmation(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("must not call AWX at all: %s %s", r.Method, r.URL.Path)
	})
	defer srv.Close()

	out, err := cancel.Execute(nil, nil, append(auth(srv.URL), str("job_id", "7")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Confirm Destructive Action"))
}

// ---------------------------------------------------------------------------
// job_relaunch
// ---------------------------------------------------------------------------

func TestRelaunchJobHappyPath(t *testing.T) {
	RegisterTestingT(t)
	var body string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/jobs/7/relaunch/"))
		if r.Method == http.MethodGet {
			// retry_counts is a DICT once the job has finished…
			_, _ = w.Write([]byte(`{"retry_counts":{"failed":2,"all":5}}`))
			return
		}
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		body = string(buf)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"job":99,"id":99,"type":"job","status":"pending"}`))
	})
	defer srv.Close()

	out, err := relaunch.Execute(nil, nil, append(auth(srv.URL),
		str("job_id", "7"), str("hosts", "failed")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["job_id"]).To(Equal("99"))
	Expect(out["job_kind"]).To(Equal("job"))
	Expect(out["status"]).To(Equal("pending"))
	Expect(out["retry_counts"]).To(Equal(map[string]interface{}{"failed": float64(2), "all": float64(5)}))
	Expect(body).To(ContainSubstring(`"hosts":"failed"`))
}

// ★ A WORKFLOW relaunch adds NO extra key: the new id is in "id". Reading it from
// a "workflow_job" key would find nothing.
func TestRelaunchWorkflowReadsTheNewIDFromTheIDKey(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/workflow_jobs/9/relaunch/"))
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed) // no GET on a workflow relaunch
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":123,"type":"workflow_job","status":"pending"}`))
	})
	defer srv.Close()

	out, err := relaunch.Execute(nil, nil, append(auth(srv.URL),
		str("job_kind", "workflow_job"), str("job_id", "9")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["job_id"]).To(Equal("123"))
	Expect(out["job_kind"]).To(Equal("workflow_job"))
	Expect(out["retry_counts"]).To(BeNil()) // the GET 405'd; that is not a failure
}

// GET /relaunch/ returns retry_counts as a STRING while the job is still active —
// type-unstable, so it must never be decoded into a map.
func TestRelaunchToleratesRetryCountsAsAString(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"retry_counts":"Relaunch by host status not available until the job finishes running."}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"job":100,"type":"job","status":"pending"}`))
	})
	defer srv.Close()

	out, err := relaunch.Execute(nil, nil, append(auth(srv.URL), str("job_id", "7")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["retry_counts"]).To(ContainSubstring("not available until the job finishes"))
}

// ---------------------------------------------------------------------------
// job_list
// ---------------------------------------------------------------------------

// ★ AWX's own default ordering on /jobs/ is id ASCENDING — the OLDEST first. The
// node must force newest-first.
func TestListJobsDefaultsToNewestFirst(t *testing.T) {
	RegisterTestingT(t)
	var q url.Values
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/jobs/"))
		q = r.URL.Query()
		_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[{"id":9},{"id":8}]}`))
	})
	defer srv.Close()

	out, err := list.Execute(nil, nil, auth(srv.URL))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(q.Get("order_by")).To(Equal("-created"))
	Expect(out["count"]).To(Equal(2))
	Expect(out["total_count"]).To(Equal(2))
	Expect(out["has_more"]).To(Equal(false))
}

func TestListJobsMapsItsFiltersAndNeverSendsLimit(t *testing.T) {
	RegisterTestingT(t)
	var q url.Values
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
	})
	defer srv.Close()

	_, err := list.Execute(nil, nil, append(auth(srv.URL),
		str("job_template_id", "5"),
		str("status", "failed"),
		str("launch_type", "scheduled"),
		str("name", "deploy"),
		str("created_after", "2026-07-01T00:00:00Z"),
		integer("page_size", 500), // over AWX's MAX_PAGE_SIZE
	))
	Expect(err).To(BeNil())
	Expect(q.Get("job_template")).To(Equal("5"))
	Expect(q.Get("status__in")).To(Equal("failed"))
	Expect(q.Get("launch_type")).To(Equal("scheduled"))
	Expect(q.Get("name__icontains")).To(Equal("deploy"))
	Expect(q.Get("created__gt")).To(Equal("2026-07-01T00:00:00Z"))
	Expect(q.Get("page_size")).To(Equal("200")) // clamped, not silently dropped
	// `limit` is a RESERVED query-param name in AWX's filter backend.
	Expect(q).ToNot(HaveKey("limit"))
}

func TestListJobsWorkflowKindUsesTheWorkflowCollection(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/workflow_jobs/"))
		// job_template is meaningless here and must not be sent.
		Expect(r.URL.Query()).ToNot(HaveKey("job_template"))
		_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
	})
	defer srv.Close()

	out, err := list.Execute(nil, nil, append(auth(srv.URL),
		str("job_kind", "workflow_job"), str("job_template_id", "5")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
}

// ---------------------------------------------------------------------------
// job_events_list
// ---------------------------------------------------------------------------

// ★ THE TRAP: ?limit= on an events endpoint silently switches AWX to
// LimitPagination and the response loses count/next/previous entirely. Page Size
// must only ever map to page_size.
func TestListJobEventsNeverSendsLimit(t *testing.T) {
	RegisterTestingT(t)
	var q url.Values
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/jobs/5/job_events/"))
		q = r.URL.Query()
		_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":1,"event":"runner_on_failed"}]}`))
	})
	defer srv.Close()

	out, err := events.Execute(nil, nil, append(auth(srv.URL),
		str("job_id", "5"),
		boolean("failed_only", true),
		boolean("no_truncate", true),
		str("host_name", "host1"),
		integer("page_size", 500),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(q).ToNot(HaveKey("limit")) // ★ the whole point
	Expect(q.Get("page_size")).To(Equal("200"))
	Expect(q.Get("failed")).To(Equal("true"))
	Expect(q.Get("no_truncate")).To(Equal("true"))
	Expect(q.Get("host_name")).To(Equal("host1"))
	Expect(out["count"]).To(Equal(1))
	Expect(out["total_count"]).To(Equal(1))
}

// Ad-hoc events live at /ad_hoc_commands/{id}/events/, NOT at a top-level
// /ad_hoc_command_events/ collection.
func TestListJobEventsAdHocUsesTheNestedEventsPath(t *testing.T) {
	RegisterTestingT(t)
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/ad_hoc_commands/5/events/"))
		_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
	})
	defer srv.Close()

	out, err := events.Execute(nil, nil, append(auth(srv.URL),
		str("job_kind", "ad_hoc_command"), str("job_id", "5")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
}
