// Tests for the schedules + directory + diagnostics family of the AWX node:
// schedule_create / schedule_update / schedule_delete / schedule_list,
// organization_list / team_list / user_list / execution_environment_list,
// and ping / me.
//
// They live in one file (in the schedule_create package, which imports the
// others) rather than nine near-identical ones, because what is worth pinning is
// the handful of traps these actions share — the RRULE pre-validation, the
// prompt-field refusal, the destructive guard, the tri-state Enabled checkbox,
// the /me/ list-not-object shape, and ping's refusal to read "AWX answered" as
// "your credential works".
package infrastructure_awx_schedule_create

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	awx "flomation.app/automate/executor/actions/infrastructure/awx"
	eelist "flomation.app/automate/executor/actions/infrastructure/awx/execution_environment_list"
	meaction "flomation.app/automate/executor/actions/infrastructure/awx/me"
	orglist "flomation.app/automate/executor/actions/infrastructure/awx/organization_list"
	pingaction "flomation.app/automate/executor/actions/infrastructure/awx/ping"
	scheddelete "flomation.app/automate/executor/actions/infrastructure/awx/schedule_delete"
	schedlist "flomation.app/automate/executor/actions/infrastructure/awx/schedule_list"
	schedupdate "flomation.app/automate/executor/actions/infrastructure/awx/schedule_update"
	teamlist "flomation.app/automate/executor/actions/infrastructure/awx/team_list"
	userlist "flomation.app/automate/executor/actions/infrastructure/awx/user_list"
	. "github.com/onsi/gomega"
)

const testToken = "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"

// bannerOnlyServer answers the unauthenticated API-root banner exactly as a real
// upstream AWX 24.6.1 does, and leaves everything else — including me/ — to h.
func bannerOnlyServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	awx.ResetAPIRootCacheForTest()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/" {
			_, _ = w.Write([]byte(`{"description":"AWX REST API","current_version":"/api/v2/","available_versions":{"v2":"/api/v2/"}}`))
			return
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// awxServer additionally answers the authenticated me/ probe that API-root
// discovery confirms the prefix with, so a test only has to handle its own
// endpoint.
func awxServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	return bannerOnlyServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/me/" {
			w.Header().Set("X-API-Product-Name", "AWX")
			w.Header().Set("X-API-Product-Version", "24.6.1")
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"username":"admin","email":"admin@example.com","is_superuser":true,"is_system_auditor":false}]}`))
			return
		}
		h(w, r)
	})
}

func inputs(base string, extra ...*core.Connection) []*core.Connection {
	in := []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: base},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: testToken},
	}
	return append(in, extra...)
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func boolean(name string, value bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: value}
}

func decode(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	if raw != "" {
		Expect(json.Unmarshal([]byte(raw), &out)).To(Succeed())
	}
	return out
}

// ---------------------------------------------------------------------------
// schedule_create
// ---------------------------------------------------------------------------

// The happy path, and the ordering that makes the rrule field usable by a
// non-technical operator: AWX's own preview endpoint validates the rule and
// returns the next ten run times BEFORE the schedule is created, and those times
// come back on the `preview` output.
func TestScheduleCreatePreviewsTheRuleThenCreates(t *testing.T) {
	RegisterTestingT(t)

	var calls []string
	var createBody string
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/api/v2/job_templates/7/launch/":
			_, _ = w.Write([]byte(`{"can_start_without_user_input":true,"survey_enabled":false,"ask_limit_on_launch":false,"variables_needed_to_start":[]}`))
		case "/api/v2/schedules/preview/":
			_, _ = w.Write([]byte(`{"local":["2026-08-01T09:00:00+01:00"],"utc":["2026-08-01T08:00:00Z"]}`))
		case "/api/v2/schedules/":
			b, _ := io.ReadAll(r.Body)
			createBody = string(b)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":12,"name":"Nightly patching","next_run":"2026-08-01T08:00:00Z","timezone":"Europe/London","enabled":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		str("name", "Nightly patching"),
		str("rrule", "DTSTART;TZID=Europe/London:20260801T090000 RRULE:FREQ=DAILY;INTERVAL=1"),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("12"))
	Expect(out["next_run"]).To(Equal("2026-08-01T08:00:00Z"))
	Expect(out["timezone"]).To(Equal("Europe/London"))

	preview, ok := out["preview"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(preview["utc"]).To(HaveLen(1))

	// The rule is previewed BEFORE anything is created.
	Expect(calls).To(Equal([]string{
		"GET /api/v2/job_templates/7/launch/",
		"POST /api/v2/schedules/preview/",
		"POST /api/v2/schedules/",
	}))

	sent := decode(t, createBody)
	Expect(sent["unified_job_template"]).To(BeEquivalentTo(7))
	Expect(sent["name"]).To(Equal("Nightly patching"))
	Expect(sent["rrule"]).To(Equal("DTSTART;TZID=Europe/London:20260801T090000 RRULE:FREQ=DAILY;INTERVAL=1"))
	// Untouched Enabled checkbox must be OMITTED, not sent as false — AWX's own
	// default is enabled, and the manifest cannot carry a default.
	Expect(sent).NotTo(HaveKey("enabled"))
}

// ★ THE TRAP. AWX hard-400s a schedule that carries a prompt field the template
// is not configured to accept. The node pre-flights and refuses first, naming the
// field — and NOTHING is created.
func TestScheduleCreateRefusesAPromptFieldTheTemplateWillNotAccept(t *testing.T) {
	RegisterTestingT(t)

	created := false
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/job_templates/7/launch/":
			_, _ = w.Write([]byte(`{"can_start_without_user_input":true,"survey_enabled":false,"ask_limit_on_launch":false}`))
		case "/api/v2/schedules/":
			created = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":13}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		str("name", "Nightly patching"),
		str("rrule", "DTSTART;TZID=Europe/London:20260801T090000 RRULE:FREQ=DAILY;INTERVAL=1"),
		str("limit", "web*"),
	))

	Expect(err).To(BeNil()) // soft failure: the flow keeps walking
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Limit"))
	Expect(created).To(BeFalse())
}

// A malformed rule is reported in AWX's own words, and no schedule is left behind.
func TestScheduleCreateSurfacesABadRecurrenceRule(t *testing.T) {
	RegisterTestingT(t)

	created := false
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/job_templates/7/launch/":
			_, _ = w.Write([]byte(`{"can_start_without_user_input":true,"survey_enabled":false}`))
		case "/api/v2/schedules/preview/":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"rrule":["A DTSTART with a valid timezone must be provided."]}`))
		case "/api/v2/schedules/":
			created = true
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		str("name", "Nightly patching"),
		str("rrule", "FREQ=DAILY"), // no DTSTART, no INTERVAL
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("DTSTART"))
	Expect(out["error"]).To(ContainSubstring("INTERVAL"))
	Expect(created).To(BeFalse())
}

// A missing required field is a SOFT failure — it must not abort the whole flow.
func TestScheduleCreateMissingRuleIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })

	out, err := Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		str("name", "Nightly patching"),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Recurrence Rule is required"))
}

// A missing credential is the ONE hard failure in the whole node.
func TestScheduleCreateMissingTokenIsAHardFailure(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: "https://awx.example.com"},
	})

	Expect(out).To(BeNil())
	Expect(err).NotTo(BeNil())
	Expect(err.Error()).To(ContainSubstring("API Token is required"))
}

// ---------------------------------------------------------------------------
// schedule_update
// ---------------------------------------------------------------------------

// Pause/resume is the point of this action, and the tri-state is the trap: an
// untouched Enabled checkbox must never be PATCHed as false.
func TestScheduleUpdatePausesAndOmitsAnUntouchedCheckbox(t *testing.T) {
	RegisterTestingT(t)

	var patched string
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/schedules/12/" && r.Method == http.MethodPatch {
			b, _ := io.ReadAll(r.Body)
			patched = string(b)
			_, _ = w.Write([]byte(`{"id":12,"name":"Nightly patching","enabled":false,"next_run":null}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	out, err := schedupdate.Execute(nil, nil, inputs(srv.URL,
		str("schedule_id", "12"),
		boolean("enabled", false),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["enabled"]).To(Equal(false))
	Expect(out["tool_result"]).To(ContainSubstring("paused"))
	Expect(decode(t, patched)).To(HaveKeyWithValue("enabled", false))

	// Renaming a schedule must not disable it.
	patched = ""
	out, err = schedupdate.Execute(nil, nil, inputs(srv.URL,
		str("schedule_id", "12"),
		str("name", "Renamed"),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	sent := decode(t, patched)
	Expect(sent).To(HaveKeyWithValue("name", "Renamed"))
	Expect(sent).NotTo(HaveKey("enabled"))
}

// A new rule is validated against AWX's preview endpoint before the PATCH, so a
// typo cannot leave the schedule half-changed.
func TestScheduleUpdateValidatesANewRuleFirst(t *testing.T) {
	RegisterTestingT(t)

	patched := false
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/schedules/preview/":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"rrule":["A DTSTART with a valid timezone must be provided."]}`))
		case r.URL.Path == "/api/v2/schedules/12/":
			patched = true
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	out, err := schedupdate.Execute(nil, nil, inputs(srv.URL,
		str("schedule_id", "12"),
		str("rrule", "FREQ=DAILY"),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("has not been changed"))
	Expect(patched).To(BeFalse())
}

func TestScheduleUpdateWithNothingToChange(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })

	out, err := schedupdate.Execute(nil, nil, inputs(srv.URL, str("schedule_id", "12")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Nothing to update"))
}

// ---------------------------------------------------------------------------
// schedule_delete
// ---------------------------------------------------------------------------

func TestScheduleDeleteRefusesWithoutTheDestructiveGuard(t *testing.T) {
	RegisterTestingT(t)

	deleted := false
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	out, err := scheddelete.Execute(nil, nil, inputs(srv.URL, str("schedule_id", "12")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Confirm Destructive Action"))
	Expect(out["error"]).To(ContainSubstring("Update Schedule")) // the non-destructive alternative
	Expect(deleted).To(BeFalse())
}

func TestScheduleDeleteWithConfirmation(t *testing.T) {
	RegisterTestingT(t)

	var path string
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			path = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	out, err := scheddelete.Execute(nil, nil, inputs(srv.URL,
		str("schedule_id", "12"),
		boolean("confirm_destructive", true),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["deleted"]).To(Equal(true))
	Expect(out["id"]).To(Equal("12"))
	Expect(path).To(Equal("/api/v2/schedules/12/")) // trailing slash — Django APPEND_SLASH
}

// ---------------------------------------------------------------------------
// schedule_list
// ---------------------------------------------------------------------------

func TestScheduleListFiltersAndDefaultsToNextRun(t *testing.T) {
	RegisterTestingT(t)

	var query string
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/schedules/" {
			query = r.URL.Query().Encode()
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":12,"name":"Nightly patching","next_run":"2026-08-01T08:00:00Z"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	out, err := schedlist.Execute(nil, nil, inputs(srv.URL,
		str("job_template_id", "7"),
		str("enabled", "true"),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(out["total_count"]).To(Equal(1))
	Expect(out["has_more"]).To(Equal(false))

	q := decodeQuery(query)
	Expect(q["unified_job_template"]).To(Equal("7")) // a schedule's FK, not job_template
	Expect(q["enabled"]).To(Equal("true"))
	Expect(q["order_by"]).To(Equal("next_run"))
	Expect(q["page_size"]).To(Equal("50"))
}

// ---------------------------------------------------------------------------
// organization_list / team_list / user_list / execution_environment_list
// ---------------------------------------------------------------------------

func TestOrganizationAndTeamListsFilter(t *testing.T) {
	RegisterTestingT(t)

	var orgQuery, teamPath, teamQuery string
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/organizations/":
			orgQuery = r.URL.Query().Encode()
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":1,"name":"Default"}]}`))
		case "/api/v2/teams/":
			teamPath, teamQuery = r.URL.Path, r.URL.Query().Encode()
			_, _ = w.Write([]byte(`{"count":0,"next":null,"results":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	out, err := orglist.Execute(nil, nil, inputs(srv.URL, str("search", "def")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(decodeQuery(orgQuery)["order_by"]).To(Equal("name"))
	Expect(decodeQuery(orgQuery)["search"]).To(Equal("def"))

	out, err = teamlist.Execute(nil, nil, inputs(srv.URL, str("organization_id", "1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(teamPath).To(Equal("/api/v2/teams/"))
	Expect(decodeQuery(teamQuery)["organization"]).To(Equal("1"))
}

// Org membership is a ROLE in AWX, not a column — ?organization= on /users/ does
// nothing, so an org-scoped user list must go to the sublist endpoint.
func TestUserListUsesTheOrganizationSublist(t *testing.T) {
	RegisterTestingT(t)

	var path, query string
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/users/" || r.URL.Path == "/api/v2/organizations/1/users/" {
			path, query = r.URL.Path, r.URL.Query().Encode()
			_, _ = w.Write([]byte(`{"count":1,"next":null,"results":[{"id":1,"username":"admin","password":"$encrypted$"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	out, err := userlist.Execute(nil, nil, inputs(srv.URL, str("is_superuser", "true")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(path).To(Equal("/api/v2/users/"))
	Expect(decodeQuery(query)["is_superuser"]).To(Equal("true"))
	Expect(decodeQuery(query)["order_by"]).To(Equal("username"))

	out, err = userlist.Execute(nil, nil, inputs(srv.URL, str("organization_id", "1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(path).To(Equal("/api/v2/organizations/1/users/"))
	Expect(out["tool_result"]).To(ContainSubstring("organization 1"))
}

// A global execution environment has organization = null. Leaving Organization
// blank must therefore send NO organization filter, or every global environment
// vanishes from the list.
func TestExecutionEnvironmentListKeepsTheGlobalsWhenOrgIsBlank(t *testing.T) {
	RegisterTestingT(t)

	var query string
	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/execution_environments/" {
			query = r.URL.Query().Encode()
			_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[{"id":1,"name":"AWX EE (24.6.1)","organization":null},{"id":3,"name":"Control Plane EE","organization":null}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	out, err := eelist.Execute(nil, nil, inputs(srv.URL))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(decodeQuery(query)).NotTo(HaveKey("organization"))

	out, err = eelist.Execute(nil, nil, inputs(srv.URL, str("organization_id", "2")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(decodeQuery(query)["organization"]).To(Equal("2"))
}

// An AWX 404 is a SOFT failure — it must never abort the flow.
func TestAListAgainstAMissingObjectIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not found."}`))
	})

	out, err := userlist.Execute(nil, nil, inputs(srv.URL, str("organization_id", "999")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("404"))
}

// ---------------------------------------------------------------------------
// ping
// ---------------------------------------------------------------------------

func TestPingReportsTheResolvedAPIRootAndTheCredential(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/ping/" {
			_, _ = w.Write([]byte(`{"ha":false,"version":"24.6.1","active_node":"awx-1","instances":[{"node":"awx-1"}],"instance_groups":[{"name":"default"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	out, err := pingaction.Execute(nil, nil, inputs(srv.URL))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["reachable"]).To(Equal(true))
	Expect(out["credential_valid"]).To(Equal(true))
	Expect(out["username"]).To(Equal("admin (superuser)"))
	Expect(out["version"]).To(Equal("24.6.1"))
	Expect(out["product"]).To(Equal("AWX"))
	// ★ the diagnostic the action exists for.
	Expect(out["api_root"]).To(Equal("/api/v2/"))
	Expect(out["active_node"]).To(Equal("awx-1"))
	Expect(out["ha"]).To(Equal(false))
	Expect(out["instances"]).To(HaveLen(1))
}

// ★ THE TRAP. On upstream AWX, ping/ is AllowAny — it answers 200 for a garbage
// token. Ping must NOT report that as a working connection: it reports reachable
// = true, credential_valid = false, and FAILS.
func TestPingDoesNotReadAReachableAWXAsAWorkingCredential(t *testing.T) {
	RegisterTestingT(t)

	srv := bannerOnlyServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/ping/": // AllowAny: answers even with a bad token
			_, _ = w.Write([]byte(`{"ha":false,"version":"24.6.1","active_node":"awx-1"}`))
		case "/api/v2/me/":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"Authentication credentials were not provided."}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	out, err := pingaction.Execute(nil, nil, inputs(srv.URL))
	Expect(err).To(BeNil()) // soft failure
	Expect(out["success"]).To(Equal(false))
	Expect(out["reachable"]).To(Equal(true))
	Expect(out["credential_valid"]).To(Equal(false))
	Expect(out["api_root"]).To(Equal("/api/v2/")) // still the most useful diagnostic
	Expect(out["version"]).To(Equal("24.6.1"))
	Expect(out["error"]).To(ContainSubstring("did not accept the credential"))
}

func TestPingReportsAnUnreachableController(t *testing.T) {
	RegisterTestingT(t)

	awx.ResetAPIRootCacheForTest()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	out, err := pingaction.Execute(nil, nil, inputs(srv.URL))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["reachable"]).To(Equal(false))
	Expect(out["credential_valid"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Could not"))
}

// ---------------------------------------------------------------------------
// me
// ---------------------------------------------------------------------------

// /me/ is a PAGINATED LIST, not an object — the user is results[0].
func TestMeUnwrapsThePaginatedList(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })

	out, err := meaction.Execute(nil, nil, inputs(srv.URL))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("1"))
	Expect(out["username"]).To(Equal("admin"))
	Expect(out["email"]).To(Equal("admin@example.com"))
	Expect(out["is_superuser"]).To(Equal(true))
	Expect(out["is_system_auditor"]).To(Equal(false))
	Expect(out["tool_result"]).To(ContainSubstring("superuser"))

	result, ok := out["result"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(result["username"]).To(Equal("admin")) // the unwrapped user, not the envelope
	Expect(result).NotTo(HaveKey("results"))
}

// decodeQuery turns an encoded query string back into a flat map for assertions.
func decodeQuery(encoded string) map[string]string {
	values, err := url.ParseQuery(encoded)
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for k := range values {
		out[k] = values.Get(k)
	}
	return out
}
