// Tests for both ad-hoc actions — Run Ad-Hoc Command and Get Ad-Hoc Command.
// They live together because they are one family and share the fixture server.
package infrastructure_awx_adhoc_command_run_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	adhocget "flomation.app/automate/executor/actions/infrastructure/awx/adhoc_command_get"
	adhocrun "flomation.app/automate/executor/actions/infrastructure/awx/adhoc_command_run"
	. "github.com/onsi/gomega"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const testToken = "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func boolean(name string, value bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: value}
}

func object(name string, value map[string]interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: value}
}

// authInputs is the credential block every test starts from: a token, pointed at
// the fixture server, with the destructive guard already ticked. A test that is
// about the guard or about a missing credential overrides these.
func authInputs(base string) []*core.Connection {
	return []*core.Connection{
		str("awx_url", base),
		str("auth_method", "token"),
		&core.Connection{Name: "api_token", Type: core.ConnectionTypeSecret, Value: testToken},
	}
}

// runModule is a valid Run Ad-Hoc Command node for a given module: inventory 3,
// machine credential 5, guard ticked.
//
// The module is a PARAMETER rather than something a caller appends, because
// core.FindConnection returns the FIRST input with a matching name — an override
// appended after a default would be silently shadowed by it.
func runModule(base, module string, extra ...*core.Connection) []*core.Connection {
	in := append(authInputs(base),
		str("inventory_id", "3"),
		str("credential_id", "5"),
		str("module_name", module),
		boolean("confirm_destructive", true),
	)
	return append(in, extra...)
}

// runInputs is runModule with the harmless default: ping.
func runInputs(base string, extra ...*core.Connection) []*core.Connection {
	return runModule(base, "ping", extra...)
}

// awxServer answers the API-root discovery probe exactly as a real upstream AWX
// 24.6.1 does and delegates everything else to h.
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

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// ---------------------------------------------------------------------------
// Run — the happy path, and the stale-status trap
// ---------------------------------------------------------------------------

// ★ THE STALE-STATUS TRAP. AWX serialises the 201 BEFORE signal_start() is
// called, so the status it reports is a stale "new". The action must re-read the
// record rather than emit what the 201 said — otherwise a downstream condition
// branches on a status that was already wrong when it was written.
func TestRunReReadsTheStaleStatusFromThe201(t *testing.T) {
	RegisterTestingT(t)

	var posted map[string]interface{}
	var detailGets int32

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/ad_hoc_commands/":
			raw, _ := io.ReadAll(r.Body)
			Expect(json.Unmarshal(raw, &posted)).To(Succeed())
			// AWX's own 201: status is the stale "new".
			writeJSON(w, http.StatusCreated, `{"id":42,"type":"ad_hoc_command","status":"new","module_name":"ping"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/ad_hoc_commands/42/":
			atomic.AddInt32(&detailGets, 1)
			writeJSON(w, http.StatusOK, `{"id":42,"type":"ad_hoc_command","status":"pending","failed":false,"module_name":"ping","module_args":""}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})
	defer srv.Close()

	out, err := adhocrun.Execute(nil, nil, runInputs(srv.URL,
		str("limit", "web*:&prod"),
		str("job_type", "check"),
		str("verbosity", "2"),
		&core.Connection{Name: "forks", Type: core.ConnectionTypeInteger, Value: 10},
		boolean("become_enabled", true),
		str("execution_environment_id", "7"),
	))

	Expect(err).To(BeNil()) // never a hard error once the credential resolves
	Expect(out["success"]).To(BeTrue())
	Expect(out["job_id"]).To(Equal("42"))
	Expect(out["job_kind"]).To(Equal(awx.JobKindAdHocCommand))
	// The status came from the re-GET, not from the 201.
	Expect(out["status"]).To(Equal("pending"))
	Expect(atomic.LoadInt32(&detailGets)).To(Equal(int32(1)))
	Expect(out["job_url"]).To(ContainSubstring("/#/jobs/command/42/output"))
	// Ad-hoc commands have no artifacts — the key must not be fabricated.
	Expect(out).NotTo(HaveKey("artifacts"))

	// The body AWX actually received: ids as JSON integers, not strings.
	Expect(posted["inventory"]).To(BeEquivalentTo(3))
	Expect(posted["credential"]).To(BeEquivalentTo(5))
	Expect(posted["module_name"]).To(Equal("ping"))
	Expect(posted["limit"]).To(Equal("web*:&prod"))
	Expect(posted["job_type"]).To(Equal("check"))
	Expect(posted["verbosity"]).To(BeEquivalentTo(2))
	Expect(posted["forks"]).To(BeEquivalentTo(10))
	Expect(posted["become_enabled"]).To(BeTrue())
	Expect(posted["execution_environment"]).To(BeEquivalentTo(7))
	// Untouched tri-state checkbox: omitted, never sent as false.
	Expect(posted).NotTo(HaveKey("diff_mode"))
	// ping takes no arguments, so module_args must not be sent as an empty string.
	Expect(posted).NotTo(HaveKey("module_args"))
}

func TestRunWaitsForCompletionAndReturnsTheOutput(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/ad_hoc_commands/":
			writeJSON(w, http.StatusCreated, `{"id":42,"type":"ad_hoc_command","status":"new"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/ad_hoc_commands/" && r.URL.Query().Get("id") == "42":
			// The hot loop polls the LIST endpoint, not the detail.
			writeJSON(w, http.StatusOK, `{"count":1,"next":null,"results":[{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/ad_hoc_commands/42/":
			writeJSON(w, http.StatusOK, `{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"elapsed":3.5,"event_processing_finished":true,"host_status_counts":{"ok":2}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/ad_hoc_commands/42/stdout/":
			Expect(r.URL.Query().Get("format")).To(Equal("txt_download"))
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("host1 | SUCCESS => {\"ping\": \"pong\"}\n"))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusTeapot)
		}
	})
	defer srv.Close()

	out, err := adhocrun.Execute(nil, nil, runInputs(srv.URL,
		boolean("wait_for_completion", true),
		boolean("include_stdout", true),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["status"]).To(Equal("successful"))
	Expect(out["finished"]).To(BeTrue())
	Expect(out["failed"]).To(BeFalse())
	Expect(out["timed_out"]).To(BeFalse())
	Expect(out["elapsed"]).To(Equal("3.5"))
	Expect(out["event_processing_finished"]).To(BeTrue())
	Expect(out["stdout"]).To(ContainSubstring("pong"))
	Expect(out["host_status_counts"]).To(Equal(map[string]interface{}{"ok": float64(2)}))
}

// A failed command is a SOFT failure that still emits the job id — the flow must
// be able to fetch the output of the run that just failed.
func TestRunSoftFailsOnAFailedCommandButKeepsTheJobID(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			writeJSON(w, http.StatusCreated, `{"id":42,"type":"ad_hoc_command","status":"new"}`)
		case r.URL.Path == "/api/v2/ad_hoc_commands/":
			writeJSON(w, http.StatusOK, `{"count":1,"results":[{"id":42,"status":"failed","finished":"2026-07-14T10:00:05Z","failed":true}]}`)
		default:
			writeJSON(w, http.StatusOK, `{"id":42,"status":"failed","finished":"2026-07-14T10:00:05Z","failed":true,"host_status_counts":{"failures":1}}`)
		}
	})
	defer srv.Close()

	out, err := adhocrun.Execute(nil, nil, runInputs(srv.URL, boolean("wait_for_completion", true)))

	Expect(err).To(BeNil()) // SOFT: a non-nil error would abort the whole flow
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("failed"))
	Expect(out["job_id"]).To(Equal("42"))
	Expect(out["failed"]).To(BeTrue())

	// …unless the operator said they did not care.
	out, err = adhocrun.Execute(nil, nil, runInputs(srv.URL,
		boolean("wait_for_completion", true),
		boolean("ignore_job_failure", true),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["failed"]).To(BeTrue())
}

// ★ HOST RESULTS ARE EVENTS-DERIVED, ALWAYS. host_status_counts is emitted for
// EVERY ad-hoc run — not only when Include Output is ticked — and AWX writes the
// events it comes from ASYNCHRONOUSLY. So the wait must settle the events even when
// Include Output is OFF; gating WaitForEvents on include_stdout left Host Results
// intermittently empty/partial for a downstream branch on host_status_counts.
func TestRunSettlesHostResultsEvenWithoutIncludeOutput(t *testing.T) {
	RegisterTestingT(t)

	var detailGets int32
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/ad_hoc_commands/":
			writeJSON(w, http.StatusCreated, `{"id":42,"type":"ad_hoc_command","status":"new"}`)
		case r.URL.Path == "/api/v2/ad_hoc_commands/" && r.URL.Query().Get("id") == "42":
			// Terminal on the very first poll…
			writeJSON(w, http.StatusOK, `{"count":1,"next":null,"results":[{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false}]}`)
		case r.URL.Path == "/api/v2/ad_hoc_commands/42/":
			// …but the events are still flushing. host_status_counts is EMPTY until
			// AWX reports event_processing_finished on the second detail GET.
			if atomic.AddInt32(&detailGets, 1) == 1 {
				writeJSON(w, http.StatusOK, `{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"event_processing_finished":false,"host_status_counts":{}}`)
				return
			}
			writeJSON(w, http.StatusOK, `{"id":42,"status":"successful","finished":"2026-07-14T10:00:05Z","failed":false,"event_processing_finished":true,"host_status_counts":{"ok":50}}`)
		default:
			// Include Output is OFF, so stdout must never be fetched.
			t.Errorf("unexpected %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusTeapot)
		}
	})
	defer srv.Close()

	// Wait for completion, but DELIBERATELY do NOT tick Include Output.
	out, err := adhocrun.Execute(nil, nil, runInputs(srv.URL,
		boolean("wait_for_completion", true),
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["event_processing_finished"]).To(BeTrue())
	// The whole point: read a moment earlier this was {}, and a downstream branch on
	// host_status_counts.failures would have concluded the command succeeded on every
	// host while the results were still pending.
	Expect(out["host_status_counts"]).To(Equal(map[string]interface{}{"ok": float64(50)}))
	Expect(atomic.LoadInt32(&detailGets)).To(BeNumerically(">=", 2))
	Expect(out["stdout"]).To(Equal("")) // Include Output off: no stdout fetched
}

// ---------------------------------------------------------------------------
// Run — the guards
// ---------------------------------------------------------------------------

// ★ THE FAMILY TRAP. module_name is matched against AWX's AD_HOC_COMMANDS setting
// by SHORT NAME, so a fully-qualified collection name is always rejected — but the
// setting is admin-editable, so the node must NOT refuse a module merely because
// it is not one of the 19 defaults.
func TestRunRejectsAFullyQualifiedModuleNameButNotAnUnfamiliarOne(t *testing.T) {
	RegisterTestingT(t)

	var calls int32
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Method == http.MethodPost {
			writeJSON(w, http.StatusCreated, `{"id":42,"type":"ad_hoc_command","status":"new"}`)
			return
		}
		writeJSON(w, http.StatusOK, `{"id":42,"status":"pending"}`)
	})
	defer srv.Close()

	// FQCN: refused client-side, before a single byte leaves the process.
	out, err := adhocrun.Execute(nil, nil, runModule(srv.URL, "ansible.builtin.shell", str("module_args", "uptime")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("short module names"))
	Expect(out["error"]).To(ContainSubstring(`"shell"`)) // suggests the short name
	Expect(atomic.LoadInt32(&calls)).To(Equal(int32(0)))

	// A module the node has never heard of is NOT refused: AD_HOC_COMMANDS is an
	// admin-editable runtime setting and an instance may legitimately allow it.
	out, err = adhocrun.Execute(nil, nil, runModule(srv.URL, "community_general_snap"))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(atomic.LoadInt32(&calls)).To(BeNumerically(">", 0))
}

func TestRunRequiresModuleArgsForCommandAndShell(t *testing.T) {
	RegisterTestingT(t)

	for _, module := range []string{"command", "shell"} {
		out, err := adhocrun.Execute(nil, nil, runModule("https://awx.invalid", module))
		Expect(err).To(BeNil())
		Expect(out["success"]).To(BeFalse(), module)
		Expect(out["error"]).To(ContainSubstring("Module Arguments is required"), module)
	}
}

func TestRunRefusesWithoutTheDestructiveConfirmation(t *testing.T) {
	RegisterTestingT(t)

	// Everything valid EXCEPT the guard — and an unresolved ${var.approved}, which
	// substitutes to the empty string, must fail closed too.
	for _, guard := range []*core.Connection{
		boolean("confirm_destructive", false),
		str("confirm_destructive", ""),
		nil,
	} {
		inputs := append(authInputs("https://awx.invalid"),
			str("inventory_id", "3"), str("credential_id", "5"), str("module_name", "shell"), str("module_args", "rm -rf /tmp/x"))
		if guard != nil {
			inputs = append(inputs, guard)
		}
		out, err := adhocrun.Execute(nil, nil, inputs)
		Expect(err).To(BeNil())
		Expect(out["success"]).To(BeFalse())
		Expect(out["error"]).To(ContainSubstring("Confirm Destructive Action"))
	}
}

func TestRunRejectsReservedAnsibleExtraVars(t *testing.T) {
	RegisterTestingT(t)

	out, err := adhocrun.Execute(nil, nil, runInputs("https://awx.invalid",
		object("extra_vars", map[string]interface{}{"ansible_user": "root", "target": "prod"})))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("ansible_user"))
}

func TestRunRequiresAnInventoryAndACredential(t *testing.T) {
	RegisterTestingT(t)

	inputs := append(authInputs("https://awx.invalid"), boolean("confirm_destructive", true))
	out, err := adhocrun.Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Inventory is required"))

	inputs = append(inputs, str("inventory_id", "3"))
	out, err = adhocrun.Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Machine Credential is required"))
}

// An AWX 4xx is a SOFT failure. A hard error would abort the entire flow run.
func TestRunSoftFailsOnAnAWXRejection(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadRequest, `{"credential":["Credential kind must be 'ssh'."]}`)
	})
	defer srv.Close()

	out, err := adhocrun.Execute(nil, nil, runInputs(srv.URL))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Credential kind must be 'ssh'"))
}

// The ONE hard failure: a missing credential is a mis-configured node.
func TestRunHardFailsOnAMissingCredential(t *testing.T) {
	RegisterTestingT(t)

	out, err := adhocrun.Execute(nil, nil, []*core.Connection{
		str("awx_url", "https://awx.example.com"),
		str("auth_method", "token"),
		str("inventory_id", "3"), str("credential_id", "5"), boolean("confirm_destructive", true),
	})
	Expect(out).To(BeNil())
	Expect(err).NotTo(BeNil())
	Expect(err.Error()).To(ContainSubstring("API Token is required"))
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func TestGetReturnsTheModuleAndTheHostResults(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodGet))
		Expect(r.URL.Path).To(Equal("/api/v2/ad_hoc_commands/42/"))
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer " + testToken))
		writeJSON(w, http.StatusOK, `{"id":42,"type":"ad_hoc_command","status":"successful","failed":false,
			"finished":"2026-07-14T10:00:05Z","module_name":"shell","module_args":"uptime",
			"host_status_counts":{"ok":2,"failures":0}}`)
	})
	defer srv.Close()

	out, err := adhocget.Execute(nil, nil, append(authInputs(srv.URL), str("adhoc_command_id", "42")))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["id"]).To(Equal("42"))
	Expect(out["status"]).To(Equal("successful"))
	Expect(out["finished"]).To(BeTrue()) // the timestamp, rendered as the boolean a flow branches on
	Expect(out["failed"]).To(BeFalse())
	Expect(out["module_name"]).To(Equal("shell"))
	Expect(out["module_args"]).To(Equal("uptime"))
	Expect(out["host_status_counts"]).To(Equal(map[string]interface{}{"ok": float64(2), "failures": float64(0)}))
	Expect(out["job_url"]).To(ContainSubstring("/#/jobs/command/42/output"))
	// The ad-hoc model has none of these — they must not be fabricated.
	for _, absent := range []string{"artifacts", "playbook", "project", "scm_revision", "job_template"} {
		Expect(out).NotTo(HaveKey(absent))
	}
}

// A running command has finished == null. It must read as not-finished, not as
// the string "<nil>".
func TestGetReportsARunningCommandAsUnfinished(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"id":42,"status":"running","failed":false,"finished":null,"module_name":"ping"}`)
	})
	defer srv.Close()

	out, err := adhocget.Execute(nil, nil, append(authInputs(srv.URL), str("adhoc_command_id", "42")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["finished"]).To(BeFalse())
	Expect(out["status"]).To(Equal("running"))
}

// AWX hides objects you cannot SEE behind a 404, so this must be a soft failure
// that says so — never a hard error, and never "it was deleted".
func TestGetSoftFailsOnAnUnknownID(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, `{"detail":"Not found."}`)
	})
	defer srv.Close()

	out, err := adhocget.Execute(nil, nil, append(authInputs(srv.URL), str("adhoc_command_id", "999")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(strings.ToLower(out["error"].(string))).To(ContainSubstring("permission"))
}

func TestGetRequiresAnID(t *testing.T) {
	RegisterTestingT(t)

	out, err := adhocget.Execute(nil, nil, authInputs("https://awx.example.com"))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Ad-Hoc Command ID is required"))
}
