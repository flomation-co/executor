// Tests for the two waiting/launching AWX actions — job_template_launch and its
// sibling job_wait, which is exercised from here because the two share every hazard
// worth pinning.
//
// The four that matter, in order of how much damage they do when they regress:
//
//  1. ★ THE IGNORED-FIELDS GUARD. AWX answers 201 and SILENTLY DROPS a prompt field
//     the template does not ask for. Setting Limit on a template with
//     ask_limit_on_launch=false would run the playbook against EVERY HOST. The node
//     must refuse WITHOUT LAUNCHING — TestLaunchRefusesAnIgnoredLimitAndNeverPosts
//     asserts no POST ever reaches the server.
//  2. ★ SURVEYS BYPASS ask_variables_on_launch. Gating survey answers on that flag
//     would make every survey template unlaunchable; NOT gating the non-survey keys
//     would let a silently-dropped variable through. Both directions are pinned.
//  3. ★ THE POLYMORPHIC 201. A sliced template hands back a WORKFLOW JOB with no
//     "job" key; polling /jobs/{id}/ for it 404s on an id that plainly exists.
//  4. The wait's terminal/timeout/failure semantics, including the event-settle gate
//     that stops us reading a half-written stdout.
package infrastructure_awx_job_template_launch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	job_wait "flomation.app/automate/executor/actions/infrastructure/awx/job_wait"
	. "github.com/onsi/gomega"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

func str(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

func boolean(name string, val bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: val}
}

func integer(name string, val int) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: val}
}

// object models an Object input: the editor stores it as a JSON STRING.
func object(name, jsonText string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: jsonText}
}

// inputs builds the seven-field credential block plus whatever the test adds.
func inputs(base string, extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		str("awx_url", base),
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}, extra...)
}

// awxServer stands up a server answering the API-root discovery probe exactly as the
// real upstream AWX 24.6.1 does, delegating everything else to h.
func awxServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	awx.ResetAPIRootCacheForTest()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			_, _ = w.Write([]byte(`{"description":"AWX REST API","current_version":"/api/v2/","available_versions":{"v2":"/api/v2/"}}`))
		case "/api/v2/me/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"username":"admin","is_superuser":true}]}`))
		default:
			h(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(awx.ResetAPIRootCacheForTest)
	return srv
}

// preflight is a job template's GET .../launch/ body. asks are the ask_*_on_launch
// flags that are ON; everything omitted is off, which is AWX's default and the case
// the ignored-fields guard exists for.
func preflight(id int, name string, surveyEnabled bool, asks ...string) string {
	body := map[string]interface{}{
		"can_start_without_user_input": true,
		"survey_enabled":               surveyEnabled,
		"variables_needed_to_start":    []string{},
		"passwords_needed_to_start":    []string{},
		"inventory_needed_to_start":    false,
		"job_template_data":            map[string]interface{}{"id": id, "name": name},
		"defaults":                     map[string]interface{}{},
	}
	for _, ask := range asks {
		body[ask] = true
	}
	raw, _ := json.Marshal(body)
	return string(raw)
}

// listOf wraps a job record in AWX's pagination envelope — what the wait's hot loop
// polls (?id=N on the LIST endpoint, never the expensive detail view).
func listOf(record string) string {
	return `{"count":1,"next":null,"previous":null,"results":[` + record + `]}`
}

func readBody(r *http.Request) map[string]interface{} {
	raw, _ := io.ReadAll(r.Body)
	out := map[string]interface{}{}
	_ = json.Unmarshal(raw, &out)
	return out
}

// ---------------------------------------------------------------------------
// Launch — the happy path
// ---------------------------------------------------------------------------

// TestLaunchSurveylessTemplate is the trivial path, modelled on the live fixture
// (template 8, no survey, no prompts): a bare {} launch that answers 201.
func TestLaunchSurveylessTemplate(t *testing.T) {
	RegisterTestingT(t)

	var posted map[string]interface{}
	var postedTo string

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/job_templates/8/launch/":
			_, _ = w.Write([]byte(preflight(8, "Demo Job Template Sans Survey", false)))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/job_templates/8/launch/":
			posted, postedTo = readBody(r), r.URL.Path
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":42,"job":42,"type":"job","status":"pending","ignored_fields":{}}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})

	out, err := Execute(nil, nil, inputs(srv.URL, str("job_template_id", "8")))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["job_id"]).To(Equal("42"))
	Expect(out["job_kind"]).To(Equal("job"))
	Expect(out["status"]).To(Equal("pending"))
	Expect(out["timed_out"]).To(BeFalse())
	Expect(out["ignored_fields"]).To(Equal(map[string]interface{}{}))
	Expect(out["tool_result"]).To(ContainSubstring("Demo Job Template Sans Survey"))

	// The trailing slash is not cosmetic: Django's APPEND_SLASH 301s a slash-less
	// POST and Go re-issues the redirect as a GET WITH NO BODY.
	Expect(postedTo).To(Equal("/api/v2/job_templates/8/launch/"))
	// Nothing the operator did not ask for.
	Expect(posted).To(Equal(map[string]interface{}{}))
}

// ---------------------------------------------------------------------------
// ★ The ignored-fields guard — the single most valuable property of the node
// ---------------------------------------------------------------------------

// TestLaunchRefusesAnIgnoredLimitAndNeverPosts is THE safety test.
//
// A template with ask_limit_on_launch=false silently drops a Limit and runs the
// playbook against EVERY HOST in the inventory. AWX still answers 201, so the only
// safe behaviour is to refuse BEFORE launching — and to have not launched at all.
func TestLaunchRefusesAnIgnoredLimitAndNeverPosts(t *testing.T) {
	RegisterTestingT(t)

	var posts atomic.Int32
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":42,"job":42,"type":"job","status":"pending","ignored_fields":{"limit":"web*"}}`))
			return
		}
		// ask_limit_on_launch is absent, i.e. OFF.
		_, _ = w.Write([]byte(preflight(7, "Demo Job Template", false)))
	})

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		str("limit", "web*"),
	))

	// SOFT failure: a nil error, so the flow keeps walking and an AI tool loop can
	// read the message and recover.
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Limit"))
	Expect(out["error"]).To(ContainSubstring("every host"))
	Expect(out["error"]).To(ContainSubstring("Allow Ignored Fields"))

	// ★ The whole point: nothing ran.
	Expect(posts.Load()).To(BeEquivalentTo(0), "the node must NOT launch a job it is about to refuse")
}

// TestLaunchWithAllowIgnoredFieldsLaunchesAnyway pins the escape hatch: an operator
// who has explicitly said "send it anyway and let AWX drop it" gets exactly that,
// with ignored_fields emitted so the flow can still see what was dropped.
func TestLaunchWithAllowIgnoredFieldsLaunchesAnyway(t *testing.T) {
	RegisterTestingT(t)

	var posts atomic.Int32
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":42,"job":42,"type":"job","status":"pending","ignored_fields":{"limit":"web*"}}`))
			return
		}
		_, _ = w.Write([]byte(preflight(7, "Demo Job Template", false)))
	})

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		str("limit", "web*"),
		boolean("allow_ignored_fields", true),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(posts.Load()).To(BeEquivalentTo(1))
	Expect(out["ignored_fields"]).To(Equal(map[string]interface{}{"limit": "web*"}))
}

// TestLaunchAllowsAPromptedLimit is the other half of the guard: when the template
// DOES prompt for a limit, the limit must actually be sent.
func TestLaunchAllowsAPromptedLimit(t *testing.T) {
	RegisterTestingT(t)

	var posted map[string]interface{}
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = readBody(r)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":42,"job":42,"type":"job","status":"pending","ignored_fields":{}}`))
			return
		}
		_, _ = w.Write([]byte(preflight(7, "Demo Job Template", false, "ask_limit_on_launch")))
	})

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		str("limit", "web*"),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(posted["limit"]).To(Equal("web*"))
}

// TestLaunchRecheckesIgnoredFieldsOnThe201 pins the belt-and-braces half: the
// template can be reconfigured BETWEEN the pre-flight and the launch, so a 201 that
// reports ignored_fields must fail the node even though the pre-flight passed — and
// must still emit the job id, because the job is now running and the operator needs
// to go and find it.
func TestLaunchRecheckesIgnoredFieldsOnThe201(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			// AWX dropped it after all.
			_, _ = w.Write([]byte(`{"id":42,"job":42,"type":"job","status":"pending","ignored_fields":{"limit":"web*"}}`))
			return
		}
		// The pre-flight says limit IS promptable...
		_, _ = w.Write([]byte(preflight(7, "Demo Job Template", false, "ask_limit_on_launch")))
	})

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		str("limit", "web*"),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("IGNORED"))
	Expect(out["ignored_fields"]).To(Equal(map[string]interface{}{"limit": "web*"}))
	// The job IS running — do not hide its id behind the failure.
	Expect(out["job_id"]).To(Equal("42"))
	Expect(out["job_url"]).To(ContainSubstring("/42/"))
}

// ---------------------------------------------------------------------------
// ★ Surveys
// ---------------------------------------------------------------------------

// surveySpec models the LIVE fixture on template 7: one multiplechoice and two
// MULTISELECTs, all required. A multiselect answer is a JSON ARRAY; a multiplechoice
// is a scalar string.
const surveySpec = `{
  "name":"","description":"",
  "spec":[
    {"variable":"stopandrebuilt","question_name":"Stop and rebuild?","type":"multiplechoice","required":true,"default":"","choices":["true","false"]},
    {"variable":"target_hosts","question_name":"Target hosts","type":"multiselect","required":true,"default":"","choices":["none","osmp-01","osmp-02","osmp-03"]},
    {"variable":"target_group","question_name":"Target group","type":"multiselect","required":true,"default":"","choices":["none","Corsham","Farnborugh","IICS","DLACS"]}
  ]}`

// surveyServer serves template 7: survey_enabled, and ask_variables_on_launch OFF —
// which is the real instance's configuration and exactly the trap. Survey answers
// must still be accepted.
func surveyServer(t *testing.T, onPost func(map[string]interface{})) *httptest.Server {
	return awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/job_templates/7/launch/":
			_, _ = w.Write([]byte(preflight(7, "Demo Job Template", true)))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/job_templates/7/survey_spec/":
			_, _ = w.Write([]byte(surveySpec))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/job_templates/7/launch/":
			if onPost != nil {
				onPost(readBody(r))
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":43,"job":43,"type":"job","status":"pending","ignored_fields":{}}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
}

// TestLaunchValidatesRequiredSurveyAnswersClientSide: the operator sees the problem
// in the editor, naming the variable, instead of an opaque AWX 400.
func TestLaunchValidatesRequiredSurveyAnswersClientSide(t *testing.T) {
	RegisterTestingT(t)

	posted := false
	srv := surveyServer(t, func(map[string]interface{}) { posted = true })

	out, err := Execute(nil, nil, inputs(srv.URL, str("job_template_id", "7")))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("stopandrebuilt"))
	Expect(out["error"]).To(ContainSubstring("target_hosts"))
	Expect(posted).To(BeFalse(), "a launch that cannot satisfy the survey must not be sent")
}

// ★ TestLaunchSurveyAnswersBypassAskVariablesOnLaunch is the survey half of the
// safety story. ask_variables_on_launch is OFF on this template, yet the survey's
// own variables must still be accepted — gating them on that flag would make every
// survey-enabled template unlaunchable.
func TestLaunchSurveyAnswersBypassAskVariablesOnLaunch(t *testing.T) {
	RegisterTestingT(t)

	var posted map[string]interface{}
	srv := surveyServer(t, func(body map[string]interface{}) { posted = body })

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		object("extra_vars", `{"stopandrebuilt":"false","target_hosts":["none"],"target_group":["none"]}`),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["job_id"]).To(Equal("43"))

	// The multiselect answers must reach AWX as JSON ARRAYS, not as strings.
	vars, ok := posted["extra_vars"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(vars["stopandrebuilt"]).To(Equal("false"))
	Expect(vars["target_hosts"]).To(Equal([]interface{}{"none"}))
	Expect(vars["target_group"]).To(Equal([]interface{}{"none"}))
}

// TestLaunchRefusesANonSurveyVariable is the other direction: a key the survey does
// NOT own genuinely does need ask_variables_on_launch, and AWX would silently drop
// it. The refusal must name the offending key, not the survey's.
func TestLaunchRefusesANonSurveyVariable(t *testing.T) {
	RegisterTestingT(t)

	posted := false
	srv := surveyServer(t, func(map[string]interface{}) { posted = true })

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		object("extra_vars", `{"stopandrebuilt":"false","target_hosts":["none"],"target_group":["none"],"rogue_var":"x"}`),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("rogue_var"))
	Expect(out["error"]).ToNot(ContainSubstring("stopandrebuilt"))
	Expect(posted).To(BeFalse())
}

// TestLaunchValidatesSurveyChoices: a multiselect answer outside the allowed choices
// is caught client-side.
func TestLaunchValidatesSurveyChoices(t *testing.T) {
	RegisterTestingT(t)

	srv := surveyServer(t, nil)

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		object("extra_vars", `{"stopandrebuilt":"false","target_hosts":["not-a-host"],"target_group":["none"]}`),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("target_hosts"))
	Expect(out["error"]).To(ContainSubstring("osmp-01"))
}

// ---------------------------------------------------------------------------
// ★ The polymorphic 201 — a sliced template returns a WORKFLOW job
// ---------------------------------------------------------------------------

// TestLaunchSlicedTemplateReturnsAWorkflowJobAndPollsIt is the un-debuggable bug this
// test exists to prevent: with job_slice_count > 1 the 201 carries {"workflow_job":
// 99, "type": "workflow_job"} and NO "job" key. A node that assumed "job" would poll
// /jobs/99/ and 404 on an id that plainly exists in the AWX UI.
func TestLaunchSlicedTemplateReturnsAWorkflowJobAndPollsIt(t *testing.T) {
	RegisterTestingT(t)

	var polled []string
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/job_templates/9/launch/":
			w.WriteHeader(http.StatusCreated)
			// NOTE: no "job" key at all.
			_, _ = w.Write([]byte(`{"id":99,"workflow_job":99,"type":"workflow_job","status":"pending","ignored_fields":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/job_templates/9/launch/":
			_, _ = w.Write([]byte(preflight(9, "Sliced Template", false, "ask_job_slice_count_on_launch")))
		case r.URL.Path == "/api/v2/workflow_jobs/":
			polled = append(polled, r.URL.Path)
			_, _ = w.Write([]byte(listOf(`{"id":99,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":5.2}`)))
		case r.URL.Path == "/api/v2/workflow_jobs/99/":
			polled = append(polled, r.URL.Path)
			_, _ = w.Write([]byte(`{"id":99,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":5.2}`))
		default:
			// ★ A poll of /api/v2/jobs/… here is the exact bug under test.
			t.Errorf("polled the wrong collection: %s %s", r.Method, r.URL.Path)
		}
	})

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "9"),
		integer("job_slice_count", 4),
		boolean("wait_for_completion", true),
		integer("timeout_seconds", 30),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["job_kind"]).To(Equal("workflow_job"), "a sliced launch produces a WORKFLOW job")
	Expect(out["job_id"]).To(Equal("99"))
	Expect(out["status"]).To(Equal("successful"))
	// The UI link must point at the workflow route, not the playbook one.
	Expect(out["job_url"]).To(ContainSubstring("/workflow/99/"))
	Expect(polled).To(ContainElement("/api/v2/workflow_jobs/"))
	Expect(polled).To(ContainElement("/api/v2/workflow_jobs/99/"))
}

// TestLaunchSlicedTemplateWithIncludeOutputExplainsTheEmptyStdout: a sliced launch
// produces a WORKFLOW job, which has NO output of its own — its stdout is always
// empty and WaitForJob never even fetches it. With Include Output ticked the operator
// would otherwise get a blank Output field and a bare "finished successfully" message
// with no clue why. The summary must explain it, exactly as Wait for Job does.
func TestLaunchSlicedTemplateWithIncludeOutputExplainsTheEmptyStdout(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/job_templates/9/launch/":
			_, _ = w.Write([]byte(preflight(9, "Sliced Template", false, "ask_job_slice_count_on_launch")))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/job_templates/9/launch/":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":99,"workflow_job":99,"type":"workflow_job","status":"pending","ignored_fields":{}}`))
		case r.URL.Path == "/api/v2/workflow_jobs/":
			_, _ = w.Write([]byte(listOf(`{"id":99,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":5.2}`)))
		case r.URL.Path == "/api/v2/workflow_jobs/99/":
			_, _ = w.Write([]byte(`{"id":99,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":5.2}`))
		default:
			// A workflow job has no stdout endpoint — it must never be fetched.
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "9"),
		integer("job_slice_count", 4),
		boolean("wait_for_completion", true),
		integer("timeout_seconds", 30),
		boolean("include_stdout", true),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["job_kind"]).To(Equal("workflow_job"))
	Expect(out["stdout"]).To(Equal("")) // a workflow job has no output of its own
	// The summary must EXPLAIN the empty Output field, not leave the operator staring
	// at a blank field beside a success message.
	Expect(out["tool_result"]).To(ContainSubstring("workflow job has no output of its own"))
}

// ---------------------------------------------------------------------------
// Launch + wait
// ---------------------------------------------------------------------------

// TestLaunchAndWaitToSuccess walks the real lifecycle — pending → running →
// successful — polling the LIST endpoint, then taking ONE detail GET for the fields
// the list view strips, and fetching the stdout with ?format=txt_download.
func TestLaunchAndWaitToSuccess(t *testing.T) {
	RegisterTestingT(t)

	var listPolls atomic.Int32
	var stdoutFormat string

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/job_templates/8/launch/":
			_, _ = w.Write([]byte(preflight(8, "Demo Job Template Sans Survey", false)))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/job_templates/8/launch/":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":42,"job":42,"type":"job","status":"pending","ignored_fields":{}}`))

		case r.URL.Path == "/api/v2/jobs/":
			// The hot loop polls the LIST endpoint (?id=N), never the detail: the
			// detail serializer runs two COUNT(*)s over the job-events table.
			Expect(r.URL.Query().Get("id")).To(Equal("42"))
			if listPolls.Add(1) == 1 {
				_, _ = w.Write([]byte(listOf(`{"id":42,"status":"running","finished":null,"failed":false}`)))
				return
			}
			_, _ = w.Write([]byte(listOf(`{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":5.2}`)))

		case r.URL.Path == "/api/v2/jobs/42/":
			_, _ = w.Write([]byte(`{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":5.2,
				"event_processing_finished":true,"artifacts":{"deployed":"yes"},
				"host_status_counts":{"ok":1},"result_traceback":"","job_explanation":""}`))

		case r.URL.Path == "/api/v2/jobs/42/stdout/":
			stdoutFormat = r.URL.Query().Get("format")
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("PLAY [all] ***\nTASK [ping] ***\nok: [localhost]\n"))

		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "8"),
		boolean("wait_for_completion", true),
		integer("poll_interval_seconds", 1),
		integer("timeout_seconds", 30),
		boolean("include_stdout", true),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["status"]).To(Equal("successful"))
	Expect(out["finished"]).To(BeTrue())
	Expect(out["failed"]).To(BeFalse())
	Expect(out["timed_out"]).To(BeFalse())
	Expect(out["elapsed"]).To(Equal("5.2"))
	Expect(out["stdout"]).To(ContainSubstring("ok: [localhost]"))
	// From the DETAIL view — these are stripped from the list response.
	Expect(out["artifacts"]).To(Equal(map[string]interface{}{"deployed": "yes"}))
	Expect(out["host_status_counts"]).To(Equal(map[string]interface{}{"ok": float64(1)}))
	Expect(out["event_processing_finished"]).To(BeTrue())

	Expect(listPolls.Load()).To(BeNumerically(">=", 2), "it must have polled until the job finished")
	// ?format=txt is capped at 1 MiB and answers 200 with an ENGLISH SENTENCE as the
	// body; txt_download is uncapped and streams from disk.
	Expect(stdoutFormat).To(Equal("txt_download"))
}

// TestLaunchAndWaitOnAFailedJob: the node fails by default when the playbook fails…
func TestLaunchAndWaitOnAFailedJob(t *testing.T) {
	RegisterTestingT(t)

	srv := failedJobServer(t, "failed", false)

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "8"),
		boolean("wait_for_completion", true),
		integer("timeout_seconds", 30),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring(`"failed"`))
	Expect(out["error"]).To(ContainSubstring("Ignore Job Failure"))
	Expect(out["status"]).To(Equal("failed"))
	Expect(out["job_id"]).To(Equal("42"))
}

// …and succeeds anyway when Ignore Job Failure is ticked.
func TestLaunchAndWaitIgnoringAFailedJob(t *testing.T) {
	RegisterTestingT(t)

	srv := failedJobServer(t, "failed", false)

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "8"),
		boolean("wait_for_completion", true),
		integer("timeout_seconds", 30),
		boolean("ignore_job_failure", true),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["status"]).To(Equal("failed"))
	Expect(out["failed"]).To(BeTrue())
}

// TestLaunchAndWaitOnACanceledJob pins the case AWX's `failed` flag alone does not
// cover: a CANCELLED job is not a success, and a flow must not carry on as though
// the playbook had run.
func TestLaunchAndWaitOnACanceledJob(t *testing.T) {
	RegisterTestingT(t)

	// failed=false, status=canceled — the shape that would slip through a naive
	// `if job.failed` check.
	srv := failedJobServer(t, "canceled", true)

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "8"),
		boolean("wait_for_completion", true),
		integer("timeout_seconds", 30),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("canceled"))
}

// failedJobServer serves a template-8 launch whose job goes terminal with the given
// status straight away. notFailed forces the `failed` flag off, to model AWX's
// cancelled-job shape.
func failedJobServer(t *testing.T, status string, notFailed bool) *httptest.Server {
	failed := "true"
	if notFailed {
		failed = "false"
	}
	record := fmt.Sprintf(`{"id":42,"status":%q,"finished":"2026-07-14T10:00:05Z","failed":%s,"elapsed":5.2}`, status, failed)
	detail := fmt.Sprintf(`{"id":42,"status":%q,"finished":"2026-07-14T10:00:05Z","failed":%s,"elapsed":5.2,
		"event_processing_finished":true,"artifacts":{},"host_status_counts":{"failures":1},
		"result_traceback":"","job_explanation":""}`, status, failed)

	return awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/job_templates/8/launch/":
			_, _ = w.Write([]byte(preflight(8, "Demo Job Template Sans Survey", false)))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/job_templates/8/launch/":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":42,"job":42,"type":"job","status":"pending","ignored_fields":{}}`))
		case r.URL.Path == "/api/v2/jobs/":
			_, _ = w.Write([]byte(listOf(record)))
		case r.URL.Path == "/api/v2/jobs/42/":
			_, _ = w.Write([]byte(detail))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
}

// ---------------------------------------------------------------------------
// Hard vs soft failure
// ---------------------------------------------------------------------------

// TestMissingCredentialIsTheOnlyHardFailure. A non-nil Go error ABORTS THE WHOLE
// FLOW, so it is reserved for a mis-configured node. Everything else — a missing
// field, an AWX 4xx, a timeout — is a soft failure the flow can route around.
func TestMissingCredentialIsTheOnlyHardFailure(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, []*core.Connection{str("awx_url", "https://awx.example.com")})
	Expect(err).To(HaveOccurred())
	Expect(out).To(BeNil())
	Expect(err.Error()).To(ContainSubstring("API Token is required"))
}

// TestMissingJobTemplateIsASoftFailure — the node is configured, the operator just
// has not picked a template yet.
func TestMissingJobTemplateIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, inputs("https://awx.example.com"))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Job Template is required"))
}

// TestAnAWX4xxIsASoftFailure: an AWX 404 must never abort the flow.
func TestAnAWX4xxIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not found."}`))
	})

	out, err := Execute(nil, nil, inputs(srv.URL, str("job_template_id", "404")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	// AWX hides objects you cannot SEE behind a 404 — never report it as "deleted".
	Expect(out["error"]).To(ContainSubstring("permission"))
}

// ---------------------------------------------------------------------------
// job_wait
// ---------------------------------------------------------------------------

// TestWaitGatesOnEventProcessingFinished is the asynchronous-events trap. A job's
// status flips to successful the instant the runner exits, but its stdout and
// artifacts may still be flushing to Postgres — reading them straight away yields
// TRUNCATED OR EMPTY results. The wait must keep polling the detail until
// event_processing_finished is true, and only then read them.
func TestWaitGatesOnEventProcessingFinished(t *testing.T) {
	RegisterTestingT(t)

	var detailGets atomic.Int32
	stdoutReadBeforeEventsSettled := false

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/jobs/":
			// Terminal on the very first poll…
			_, _ = w.Write([]byte(listOf(`{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":5.2}`)))

		case "/api/v2/jobs/42/":
			// …but AWX is still writing the events. Only the SECOND detail GET says
			// they are done.
			if detailGets.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":5.2,
					"event_processing_finished":false,"artifacts":{},"host_status_counts":{}}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":5.2,
				"event_processing_finished":true,"artifacts":{"deployed":"yes"},"host_status_counts":{"ok":1}}`))

		case "/api/v2/jobs/42/stdout/":
			if detailGets.Load() < 2 {
				stdoutReadBeforeEventsSettled = true
			}
			_, _ = w.Write([]byte("PLAY RECAP ***\nlocalhost : ok=1\n"))

		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})

	out, err := job_wait.Execute(nil, nil, inputs(srv.URL,
		str("job_id", "42"),
		integer("timeout_seconds", 30),
		boolean("include_stdout", true),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["status"]).To(Equal("successful"))
	Expect(out["event_processing_finished"]).To(BeTrue())
	// The artifacts are the WHOLE point: read a moment earlier they were empty, and
	// a downstream node would have silently seen nothing.
	Expect(out["artifacts"]).To(Equal(map[string]interface{}{"deployed": "yes"}))
	Expect(out["stdout"]).To(ContainSubstring("PLAY RECAP"))
	Expect(stdoutReadBeforeEventsSettled).To(BeFalse(), "stdout must not be read until AWX has finished writing the events")
	Expect(detailGets.Load()).To(BeNumerically(">=", 2))
}

// TestWaitTimesOutWithoutCancelling. On a timeout the job is left ALONE unless Cancel
// on Timeout is ticked — silently killing a production job because a flow got bored
// is surprising and destructive. The node soft-fails with the id so the operator can
// go and find it.
func TestWaitTimesOutWithoutCancelling(t *testing.T) {
	RegisterTestingT(t)

	var cancels atomic.Int32
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel/"):
			cancels.Add(1)
			w.WriteHeader(http.StatusAccepted)
		case r.URL.Path == "/api/v2/jobs/":
			_, _ = w.Write([]byte(listOf(`{"id":42,"status":"running","finished":null,"failed":false}`)))
		case r.URL.Path == "/api/v2/jobs/42/":
			_, _ = w.Write([]byte(`{"id":42,"status":"running","finished":null,"failed":false}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})

	out, err := job_wait.Execute(nil, nil, inputs(srv.URL,
		str("job_id", "42"),
		integer("timeout_seconds", 1),
		integer("poll_interval_seconds", 1),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["timed_out"]).To(BeTrue())
	Expect(out["error"]).To(ContainSubstring("Timed out"))
	Expect(out["error"]).To(ContainSubstring("still running"))
	Expect(out["job_id"]).To(Equal("42"))
	Expect(out["job_url"]).To(ContainSubstring("/42/"))
	Expect(cancels.Load()).To(BeEquivalentTo(0), "the job must NOT be cancelled unless Cancel on Timeout is ticked")
}

// TestWaitCancelsOnTimeoutWhenAsked — the opposite half.
func TestWaitCancelsOnTimeoutWhenAsked(t *testing.T) {
	RegisterTestingT(t)

	var cancels atomic.Int32
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/jobs/42/cancel/":
			cancels.Add(1)
			w.WriteHeader(http.StatusAccepted) // 202, and a COMPLETELY EMPTY body
		case r.URL.Path == "/api/v2/jobs/":
			_, _ = w.Write([]byte(listOf(`{"id":42,"status":"running","finished":null,"failed":false}`)))
		case r.URL.Path == "/api/v2/jobs/42/":
			_, _ = w.Write([]byte(`{"id":42,"status":"canceled","finished":null,"failed":false}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})

	out, err := job_wait.Execute(nil, nil, inputs(srv.URL,
		str("job_id", "42"),
		integer("timeout_seconds", 1),
		integer("poll_interval_seconds", 1),
		boolean("cancel_on_timeout", true),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["timed_out"]).To(BeTrue())
	Expect(cancels.Load()).To(BeEquivalentTo(1))
	Expect(out["error"]).To(ContainSubstring("cancelled in AWX"))
}

// TestWaitOnAnAdHocCommandUsesTheRightCollection — one Wait action serves all five of
// AWX's unified-job kinds, and each lives in its own collection.
func TestWaitOnAnAdHocCommandUsesTheRightCollection(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/ad_hoc_commands/":
			_, _ = w.Write([]byte(listOf(`{"id":7,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":2.0}`)))
		case "/api/v2/ad_hoc_commands/7/":
			_, _ = w.Write([]byte(`{"id":7,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":2.0,"event_processing_finished":true}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})

	out, err := job_wait.Execute(nil, nil, inputs(srv.URL,
		str("job_kind", "ad_hoc_command"),
		str("job_id", "7"),
		integer("timeout_seconds", 30),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["job_kind"]).To(Equal("ad_hoc_command"))
	Expect(out["tool_result"]).To(ContainSubstring("ad-hoc command 7"))
}

// TestWaitDecodesNoLogArtifacts. job.artifacts is EITHER a JSON object OR the literal
// string "$hidden due to Ansible no_log flag$" — unmarshalling straight into a map
// fails on any job whose set_stats was no_log.
func TestWaitDecodesNoLogArtifacts(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/jobs/":
			_, _ = w.Write([]byte(listOf(`{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":1.0}`)))
		case "/api/v2/jobs/42/":
			_, _ = w.Write([]byte(`{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":1.0,
				"event_processing_finished":true,"artifacts":"$hidden due to Ansible no_log flag$"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})

	out, err := job_wait.Execute(nil, nil, inputs(srv.URL,
		str("job_id", "42"),
		integer("timeout_seconds", 30),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["artifacts"]).To(Equal("$hidden due to Ansible no_log flag$"))
}

// TestWaitOnAMissingJobIsASoftFailure — a bad id must not abort the flow.
func TestWaitOnAMissingJobIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"count":0,"next":null,"previous":null,"results":[]}`))
	})

	out, err := job_wait.Execute(nil, nil, inputs(srv.URL,
		str("job_id", "999"),
		integer("timeout_seconds", 5),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("999"))
}
