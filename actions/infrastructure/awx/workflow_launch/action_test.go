package infrastructure_awx_workflow_launch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	. "github.com/onsi/gomega"
)

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

func auth(base string) []*core.Connection {
	return []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: base},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}
}

func with(base string, extra ...*core.Connection) []*core.Connection {
	return append(auth(base), extra...)
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func boolean(name string, value bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: value}
}

func obj(name string, value interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: value}
}

// preflight is the GET {root}workflow_job_templates/{id}/launch/ body. Only the
// SEVEN ask_* flags a workflow job template actually has are ever present.
func preflight(ask ...string) string {
	flags := map[string]bool{
		"ask_inventory_on_launch":  false,
		"ask_limit_on_launch":      false,
		"ask_scm_branch_on_launch": false,
		"ask_labels_on_launch":     false,
		"ask_tags_on_launch":       false,
		"ask_skip_tags_on_launch":  false,
		"ask_variables_on_launch":  false,
	}
	for _, f := range ask {
		flags[f] = true
	}
	body := map[string]interface{}{
		"can_start_without_user_input": true,
		"survey_enabled":               false,
		"variables_needed_to_start":    []string{},
		"passwords_needed_to_start":    []string{},
		"defaults":                     map[string]interface{}{},
	}
	for k, v := range flags {
		body[k] = v
	}
	raw, _ := json.Marshal(body)
	return string(raw)
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestLaunchWorkflowWithoutWaiting(t *testing.T) {
	RegisterTestingT(t)

	var launchBody map[string]interface{}
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_job_templates/3/launch/":
			_, _ = w.Write([]byte(preflight("ask_variables_on_launch", "ask_limit_on_launch")))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/workflow_job_templates/3/launch/":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &launchBody)
			Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"workflow_job":99,"id":99,"type":"workflow_job","status":"pending","ignored_fields":{}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(srv.URL,
		str("workflow_template_id", "3"),
		obj("extra_vars", `{"target_env":"prod"}`),
		str("limit", "web*"),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["workflow_job_id"]).To(Equal("99"))
	Expect(out["status"]).To(Equal("pending"))
	Expect(out["timed_out"]).To(Equal(false))
	Expect(out["job_url"]).To(ContainSubstring("/#/jobs/workflow/99/output"))

	// Only the seven a workflow can prompt for — and only the two that were set.
	Expect(launchBody).To(HaveKeyWithValue("limit", "web*"))
	Expect(launchBody).To(HaveKeyWithValue("extra_vars", map[string]interface{}{"target_env": "prod"}))
	Expect(launchBody).NotTo(HaveKey("credentials"))
	Expect(launchBody).NotTo(HaveKey("verbosity"))
}

// ---------------------------------------------------------------------------
// ★ THE TRAP: a prompt field the workflow does not accept
// ---------------------------------------------------------------------------

// ★ A WORKFLOW does not silently drop a non-prompted field the way a job template
// does — verified against a live AWX 24.6.1:
//
//	POST /api/v2/workflow_job_templates/9/launch/ {"limit":"localhost"}
//	  -> 400 {"limit":["Field is not configured to prompt on launch."]}
//
// (WorkflowJobLaunchSerializer does not pass _exclude_errors=['prompts'], unlike
// JobTemplateLaunch.post.) So the node still refuses BEFORE launching — but the
// message must say AWX will REJECT the launch, not that it would ignore the field
// and run against every host. That sentence belongs to the job template only.
func TestLaunchRefusesAPromptTheWorkflowWouldIgnore(t *testing.T) {
	RegisterTestingT(t)

	launched := false
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_job_templates/3/launch/":
			// ask_limit_on_launch is OFF.
			_, _ = w.Write([]byte(preflight("ask_variables_on_launch")))
		case r.Method == http.MethodPost:
			launched = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"workflow_job":99,"type":"workflow_job","ignored_fields":{"limit":"web*"}}`))
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(srv.URL,
		str("workflow_template_id", "3"),
		str("limit", "web*"),
	))

	Expect(err).To(BeNil(), "an AWX-side problem is a SOFT failure — a Go error would abort the whole flow")
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Limit"))
	Expect(out["error"]).To(ContainSubstring("REFUSES"),
		"a workflow launch is rejected outright — the refusal must not promise a silent drop")
	Expect(out["error"]).NotTo(ContainSubstring("every host"),
		"that is the JOB TEMPLATE's blast radius; a rejected workflow launch runs nothing at all")
	Expect(out["error"]).NotTo(ContainSubstring("Allow Ignored Fields"),
		"there is no escape hatch on a workflow — AWX will not take the launch however we ask")
	Expect(launched).To(BeFalse(), "the node must refuse BEFORE the workflow runs, not after")
}

// There is NO Allow Ignored Fields input on this action, and the fail-closed
// pre-flight must hold even if something sets one anyway: AWX answers 400 to a
// workflow launch carrying a non-prompted field, so "send it anyway" could only
// ever swap our actionable message for AWX's terse one.
func TestLaunchHasNoAllowIgnoredFieldsEscapeHatch(t *testing.T) {
	RegisterTestingT(t)

	for _, in := range Inputs {
		Expect(in.Name).NotTo(Equal("allow_ignored_fields"),
			"a workflow cannot ignore a prompt field — AWX rejects the launch")
	}

	launched := false
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_job_templates/3/launch/":
			_, _ = w.Write([]byte(preflight("ask_variables_on_launch")))
		case r.Method == http.MethodPost:
			launched = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"workflow_job":99,"type":"workflow_job","status":"pending"}`))
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(srv.URL,
		str("workflow_template_id", "3"),
		str("limit", "web*"),
		boolean("allow_ignored_fields", true), // ignored: the input does not exist
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(launched).To(BeFalse(), "ticking a non-existent hatch must not launch the workflow")
}

// The workflow can be reconfigured between the pre-flight and the launch, so the
// 201's ignored_fields is re-checked — and the id is still reported, because the
// workflow IS now running.
func TestLaunchFailsWhenThe201ReportsIgnoredFields(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_job_templates/3/launch/":
			// The pre-flight says limit IS promptable…
			_, _ = w.Write([]byte(preflight("ask_limit_on_launch")))
		case r.Method == http.MethodPost:
			// …but by the time we launch, AWX drops it anyway.
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"workflow_job":99,"type":"workflow_job","status":"pending","ignored_fields":{"limit":"web*"}}`))
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(srv.URL,
		str("workflow_template_id", "3"),
		str("limit", "web*"),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("IGNORED"))
	Expect(out["workflow_job_id"]).To(Equal("99"), "the workflow is running — the operator needs its number")
}

// ---------------------------------------------------------------------------
// Surveys
// ---------------------------------------------------------------------------

// A survey answer bypasses ask_variables_on_launch entirely, and a missing
// required answer is caught client-side rather than coming back as an AWX 400.
func TestLaunchValidatesTheSurveyBeforeLaunching(t *testing.T) {
	RegisterTestingT(t)

	launched := false
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_job_templates/3/launch/":
			_, _ = w.Write([]byte(`{"survey_enabled":true,"can_start_without_user_input":false,
				"ask_variables_on_launch":false,"variables_needed_to_start":["target_env"],
				"passwords_needed_to_start":[],"defaults":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_job_templates/3/survey_spec/":
			_, _ = w.Write([]byte(`{"name":"","description":"","spec":[
				{"variable":"target_env","question_name":"Which environment?","type":"multiplechoice","required":true,"choices":"dev\nprod"}]}`))
		case r.Method == http.MethodPost:
			launched = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"workflow_job":99,"type":"workflow_job","status":"pending","ignored_fields":{}}`))
		}
	})
	defer srv.Close()

	// Missing the required survey answer.
	out, err := Execute(nil, nil, with(srv.URL, str("workflow_template_id", "3")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("target_env"))
	Expect(launched).To(BeFalse())

	// Answered — and it launches, even though ask_variables_on_launch is FALSE.
	out, err = Execute(nil, nil, with(srv.URL,
		str("workflow_template_id", "3"),
		obj("extra_vars", `{"target_env":"prod"}`),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(launched).To(BeTrue())
	Expect(out["workflow_job_id"]).To(Equal("99"))
}

// ---------------------------------------------------------------------------
// Waiting + node results
// ---------------------------------------------------------------------------

func TestLaunchWaitsAndReturnsNodeResults(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_job_templates/3/launch/":
			_, _ = w.Write([]byte(preflight()))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/workflow_job_templates/3/launch/":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"workflow_job":99,"type":"workflow_job","status":"pending","ignored_fields":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_jobs/":
			// The hot poll uses the LIST endpoint, never the expensive detail view.
			Expect(r.URL.Query().Get("id")).To(Equal("99"))
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[
				{"id":99,"status":"successful","failed":false,"finished":"2026-07-14T10:00:05Z","elapsed":5.5}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_jobs/99/":
			_, _ = w.Write([]byte(`{"id":99,"status":"successful","failed":false,"finished":"2026-07-14T10:00:05Z","elapsed":5.5}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_jobs/99/workflow_nodes/":
			_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[
				{"id":11,"do_not_run":false,"summary_fields":{"job":{"id":100,"name":"Deploy","type":"job","status":"successful","failed":false,"elapsed":3.1}}},
				{"id":12,"do_not_run":true,"job":null,"summary_fields":{"unified_job_template":{"name":"Rollback"}}}]}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(srv.URL,
		str("workflow_template_id", "3"),
		boolean("wait_for_completion", true),
		boolean("include_node_results", true),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["status"]).To(Equal("successful"))
	Expect(out["finished"]).To(Equal(true))
	Expect(out["failed"]).To(Equal(false))
	Expect(out["timed_out"]).To(Equal(false))
	Expect(out["elapsed"]).To(Equal("5.5"))

	nodes, ok := out["node_results"].([]interface{})
	Expect(ok).To(BeTrue())
	Expect(nodes).To(HaveLen(2))

	ran := nodes[0].(map[string]interface{})
	Expect(ran["job_id"]).To(Equal("100"))
	Expect(ran["job_type"]).To(Equal("job"))
	Expect(ran["status"]).To(Equal("successful"))

	// A branch that was NOT taken has no child job — and that is not a failure.
	skipped := nodes[1].(map[string]interface{})
	Expect(skipped["do_not_run"]).To(Equal(true))
	Expect(skipped["job_id"]).To(Equal(""))
	Expect(skipped["job_name"]).To(Equal("Rollback"), "a not-taken branch is still named, from its template")
}

func TestLaunchSoftFailsOnAFailedWorkflow(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/workflow_job_templates/3/launch/":
			_, _ = w.Write([]byte(preflight()))
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"workflow_job":99,"type":"workflow_job","status":"pending","ignored_fields":{}}`))
		case r.URL.Path == "/api/v2/workflow_jobs/":
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[
				{"id":99,"status":"failed","failed":true,"finished":"2026-07-14T10:00:05Z","elapsed":5}]}`))
		case r.URL.Path == "/api/v2/workflow_jobs/99/":
			_, _ = w.Write([]byte(`{"id":99,"status":"failed","failed":true,"finished":"2026-07-14T10:00:05Z","elapsed":5}`))
		}
	})
	defer srv.Close()

	base := func(extra ...*core.Connection) []*core.Connection {
		return with(srv.URL, append([]*core.Connection{
			str("workflow_template_id", "3"),
			boolean("wait_for_completion", true),
		}, extra...)...)
	}

	out, err := Execute(nil, nil, base())
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["failed"]).To(Equal(true))
	Expect(out["status"]).To(Equal("failed"))
	Expect(out["workflow_job_id"]).To(Equal("99"))

	// Ignore Workflow Failure turns the same run into a success.
	out, err = Execute(nil, nil, base(boolean("ignore_job_failure", true)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["failed"]).To(Equal(true))
}

// ---------------------------------------------------------------------------
// Failure contract
// ---------------------------------------------------------------------------

// A missing credential is the ONE hard failure: nil outputs, non-nil error.
func TestMissingTokenIsAHardFailure(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, []*core.Connection{
		str("awx_url", "https://awx.example.com"),
		str("workflow_template_id", "3"),
	})
	Expect(out).To(BeNil())
	Expect(err).NotTo(BeNil())
	Expect(err.Error()).To(ContainSubstring("API Token is required"))
}

// A missing required field is a SOFT failure — the flow keeps walking.
func TestMissingWorkflowTemplateIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, auth("https://awx.example.com"))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Workflow Template is required"))
}

// An AWX 404 must NOT abort the flow.
func TestAWXNotFoundIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"detail":"Not found."}`)
	})
	defer srv.Close()

	out, err := Execute(nil, nil, with(srv.URL, str("workflow_template_id", "404")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("404"))
}
