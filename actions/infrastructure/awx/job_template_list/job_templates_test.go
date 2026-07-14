// Tests for the four job-template READ actions: list, get, survey_spec and
// launch options. They live together because they share one fake AWX and one
// set of fixtures, and because the traps they guard are the same family:
//
//   - a template you can SEE is often one you cannot LAUNCH
//     (summary_fields.user_capabilities.start), and an AI agent must be told;
//   - a template with NO SURVEY answers 200 with {} — not a 404 — and must be
//     reported as "no survey", not as an empty success;
//   - every AWX failure is a SOFT failure: an ErrorResult with a NIL Go error, so
//     a 404 does not abort the whole flow.
package infrastructure_awx_job_template_list

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	get "flomation.app/automate/executor/actions/infrastructure/awx/job_template_get"
	launchopts "flomation.app/automate/executor/actions/infrastructure/awx/job_template_launch_options_get"
	survey "flomation.app/automate/executor/actions/infrastructure/awx/job_template_survey_get"
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

// authInputs is the credential every test uses, plus whatever the action needs.
func authInputs(base string, extra ...*core.Connection) []*core.Connection {
	in := []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: base},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}
	return append(in, extra...)
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// twoTemplates: 7 is launchable by this user, 9 is NOT. That asymmetry is the
// whole point of the list action's summary.
const twoTemplates = `{
  "count": 2, "next": null, "previous": null,
  "results": [
    {"id": 7, "name": "Deploy web", "playbook": "site.yml",
     "summary_fields": {"user_capabilities": {"start": true, "edit": true}}},
    {"id": 9, "name": "Restore database", "playbook": "restore.yml",
     "summary_fields": {"user_capabilities": {"start": false, "edit": false}}}
  ]
}`

// ---------------------------------------------------------------------------
// job_template_list
// ---------------------------------------------------------------------------

func TestJobTemplateListReturnsTemplatesAndFilters(t *testing.T) {
	RegisterTestingT(t)

	var gotQuery string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/job_templates/"))
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(twoTemplates))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, authInputs(srv.URL,
		str("search", "deploy"),
		str("project_id", "3"),
		str("inventory_id", "4"),
	))

	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["count"]).To(Equal(2))
	Expect(out["total_count"]).To(Equal(2))
	Expect(out["has_more"]).To(BeFalse())
	Expect(out["results"]).To(HaveLen(2))

	// The filters are mapped onto AWX's own query-param names, and the default
	// ordering is by name (AWX's own default is by id).
	Expect(gotQuery).To(ContainSubstring("search=deploy"))
	Expect(gotQuery).To(ContainSubstring("project=3"))
	Expect(gotQuery).To(ContainSubstring("inventory=4"))
	Expect(gotQuery).To(ContainSubstring("order_by=name"))
	Expect(gotQuery).To(ContainSubstring("page_size=50"))
}

// ★ THE TRAP. A template the credential cannot start is listed by AWX exactly
// like one it can. If the summary does not say so, an AI agent picks the
// un-launchable one and only finds out at the 403.
func TestJobTemplateListSurfacesLaunchability(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(twoTemplates))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, authInputs(srv.URL))
	Expect(err).ToNot(HaveOccurred())

	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring(`7 "Deploy web" (launchable)`))
	Expect(summary).To(ContainSubstring(`9 "Restore database" (NOT launchable by this AWX user)`))
	Expect(summary).To(ContainSubstring("1 of them are NOT launchable"))
}

// A missing summary_fields (an older AWX, or a trimmed serializer) must read as
// NOT launchable — the safe direction.
func TestJobTemplateListTreatsAbsentCapabilitiesAsNotLaunchable(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"count":1,"results":[{"id":7,"name":"Deploy web"}]}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, authInputs(srv.URL))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["tool_result"]).To(ContainSubstring("NOT launchable by this AWX user"))
}

// SOFT FAILURE. A 403 is an ErrorResult with a NIL Go error — a non-nil error
// would abort the entire flow rather than letting the next node handle it.
func TestJobTemplateListSoftFailsOnAWXError(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"You do not have permission to perform this action."}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, authInputs(srv.URL))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("403"))
}

// GetAuth is the ONE hard error: a missing token is a mis-configured node, not a
// failed request.
func TestJobTemplateListHardFailsWithoutACredential(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: "https://awx.example.com"},
	})
	Expect(err).To(HaveOccurred())
	Expect(out).To(BeNil())
}

// ---------------------------------------------------------------------------
// job_template_get
// ---------------------------------------------------------------------------

func TestJobTemplateGet(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/job_templates/7/"))
		_, _ = w.Write([]byte(`{
		  "id": 7, "name": "Deploy web", "playbook": "site.yml",
		  "survey_enabled": true,
		  "ask_limit_on_launch": true, "ask_variables_on_launch": true,
		  "ask_inventory_on_launch": false,
		  "summary_fields": {"user_capabilities": {"start": true}}
		}`))
	})
	defer srv.Close()

	out, err := get.Execute(nil, nil, authInputs(srv.URL, str("job_template_id", "7")))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["id"]).To(Equal("7"))
	Expect(out["name"]).To(Equal("Deploy web"))
	Expect(out["playbook"]).To(Equal("site.yml"))
	Expect(out["survey_enabled"]).To(BeTrue())
	Expect(out["can_launch"]).To(BeTrue())

	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring("Limit"))
	Expect(summary).To(ContainSubstring("Variables"))
	Expect(summary).ToNot(ContainSubstring("Inventory")) // ask_inventory_on_launch is off
	Expect(summary).To(ContainSubstring("This AWX user may launch it"))
}

func TestJobTemplateGetWarnsWhenTheUserCannotLaunchIt(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":9,"name":"Restore database","playbook":"restore.yml",
		  "summary_fields":{"user_capabilities":{"start":false}}}`))
	})
	defer srv.Close()

	out, err := get.Execute(nil, nil, authInputs(srv.URL, str("job_template_id", "9")))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["can_launch"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("may NOT launch it"))
}

// A 404 must not be reported as "deleted" — AWX hides objects you cannot SEE
// behind a 404 — and must be a soft failure.
func TestJobTemplateGetSoftFailsOn404(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not found."}`))
	})
	defer srv.Close()

	out, err := get.Execute(nil, nil, authInputs(srv.URL, str("job_template_id", "404")))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("permission to see it"))
}

// A missing required id is a soft failure too: the operator gets a message, the
// flow does not die.
func TestJobTemplateGetSoftFailsWithoutAnID(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) { t.Fatalf("must not call AWX") })
	defer srv.Close()

	out, err := get.Execute(nil, nil, authInputs(srv.URL))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Job Template is required"))
}

// ---------------------------------------------------------------------------
// job_template_survey_get
// ---------------------------------------------------------------------------

func TestJobTemplateSurveyGet(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/job_templates/7/survey_spec/"))
		// choices arrives as a NEWLINE-SEPARATED STRING, not an array.
		_, _ = w.Write([]byte(`{
		  "name": "Deploy options", "description": "",
		  "spec": [
		    {"variable":"target_env","question_name":"Which environment?","type":"multiplechoice",
		     "required":true,"default":"","choices":"dev\nstaging\nprod"},
		    {"variable":"replicas","question_name":"How many?","type":"integer",
		     "required":true,"default":2,"min":1,"max":10},
		    {"variable":"vault_pass","question_name":"Vault password","type":"password",
		     "required":false,"default":"$encrypted$"}
		  ]
		}`))
	})
	defer srv.Close()

	out, err := survey.Execute(nil, nil, authInputs(srv.URL, str("job_template_id", "7")))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["has_survey"]).To(BeTrue())
	Expect(out["question_count"]).To(Equal(3))

	// Only target_env is required-with-no-default: replicas defaults to 2.
	Expect(out["required_variables"]).To(Equal([]interface{}{"target_env"}))

	spec, ok := out["spec"].([]interface{})
	Expect(ok).To(BeTrue())
	Expect(spec).To(HaveLen(3))

	// ★ choices is re-emitted as a real ARRAY so a Loop node can iterate it.
	first, _ := spec[0].(map[string]interface{})
	Expect(first["variable"]).To(Equal("target_env"))
	Expect(first["choices"]).To(Equal([]interface{}{"dev", "staging", "prod"}))

	second, _ := spec[1].(map[string]interface{})
	Expect(second["min"]).To(Equal(float64(1)))
	Expect(second["max"]).To(Equal(float64(10)))

	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring("dev / staging / prod"))
	Expect(summary).To(ContainSubstring("MUST be answered: target_env"))
	Expect(summary).To(ContainSubstring("$encrypted$"))
}

// ★ THE TRAP. No survey configured = HTTP 200 with an EMPTY OBJECT. It is not a
// 404 and it is not an error: say "no survey" in words, keep success=true, and
// never leave the operator staring at an empty result wondering what broke.
func TestJobTemplateSurveyGetReportsNoSurveyRatherThanAnEmptyObject(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	defer srv.Close()

	out, err := survey.Execute(nil, nil, authInputs(srv.URL, str("job_template_id", "9")))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["error"]).To(Equal(""))
	Expect(out["has_survey"]).To(BeFalse())
	Expect(out["question_count"]).To(Equal(0))
	Expect(out["spec"]).To(Equal([]interface{}{}))
	Expect(out["required_variables"]).To(Equal([]interface{}{}))
	Expect(out["tool_result"]).To(ContainSubstring("NO SURVEY configured"))
}

// ---------------------------------------------------------------------------
// job_template_launch_options_get
// ---------------------------------------------------------------------------

func TestJobTemplateLaunchOptionsGet(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/job_templates/7/launch/"))
		_, _ = w.Write([]byte(`{
		  "can_start_without_user_input": false,
		  "passwords_needed_to_start": ["ssh_password"],
		  "variables_needed_to_start": ["target_env"],
		  "survey_enabled": true,
		  "inventory_needed_to_start": false,
		  "credential_needed_to_start": false,
		  "ask_limit_on_launch": true,
		  "ask_variables_on_launch": true,
		  "ask_tags_on_launch": true,
		  "ask_inventory_on_launch": false,
		  "ask_verbosity_on_launch": false,
		  "defaults": {"limit": "", "job_tags": ""}
		}`))
	})
	defer srv.Close()

	out, err := launchopts.Execute(nil, nil, authInputs(srv.URL, str("job_template_id", "7")))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["survey_enabled"]).To(BeTrue())
	Expect(out["inventory_needed_to_start"]).To(BeFalse())
	Expect(out["can_start_without_user_input"]).To(BeFalse())

	// The 16 ask_* flags reduced to the BODY-FIELD names that are ON — note
	// ask_tags_on_launch → job_tags and ask_variables_on_launch → extra_vars, the
	// two that do not follow the ask_X → X rule.
	Expect(out["promptable_fields"]).To(Equal([]interface{}{"extra_vars", "job_tags", "limit"}))
	Expect(out["variables_needed_to_start"]).To(Equal([]interface{}{"target_env"}))
	Expect(out["passwords_needed_to_start"]).To(Equal([]interface{}{"ssh_password"}))

	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring("MUST be answered: target_env"))
	Expect(summary).To(ContainSubstring("Credential Passwords with: ssh_password"))
	// ★ can_start_without_user_input=false is NOT "launch will fail".
	Expect(summary).To(ContainSubstring("does NOT mean the launch will fail"))
}

// A template that prompts for nothing must say so loudly: every override the
// operator sets on the Launch node would be silently dropped by AWX.
func TestJobTemplateLaunchOptionsGetWarnsWhenNothingIsPromptable(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"can_start_without_user_input": true, "survey_enabled": false,
		  "passwords_needed_to_start": [], "variables_needed_to_start": [], "defaults": {}}`))
	})
	defer srv.Close()

	out, err := launchopts.Execute(nil, nil, authInputs(srv.URL, str("job_template_id", "7")))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["promptable_fields"]).To(Equal([]interface{}{}))
	Expect(out["tool_result"]).To(ContainSubstring("prompts for NOTHING at launch"))
}
