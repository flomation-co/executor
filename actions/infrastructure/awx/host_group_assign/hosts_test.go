// Cross-action tests for the six AWX host actions.
//
// They live in host_group_assign/ because that action carries the family's
// nastiest trap — the disassociate PRESENCE check — and the test that pins it
// (TestAddNeverSendsTheDisassociateKey) is the single most important test in the
// family. The other five actions are exercised from the same file so one
// httptest fixture serves the lot.
//
// The package is the external test package so it can import all six action
// packages without an import cycle.
package infrastructure_awx_host_group_assign_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"

	host_create "flomation.app/automate/executor/actions/infrastructure/awx/host_create"
	host_delete "flomation.app/automate/executor/actions/infrastructure/awx/host_delete"
	host_get "flomation.app/automate/executor/actions/infrastructure/awx/host_get"
	host_group_assign "flomation.app/automate/executor/actions/infrastructure/awx/host_group_assign"
	host_list "flomation.app/automate/executor/actions/infrastructure/awx/host_list"
	host_update "flomation.app/automate/executor/actions/infrastructure/awx/host_update"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// capture records what the action actually put on the wire.
type capture struct {
	Method string
	Path   string
	Query  map[string][]string
	Body   map[string]interface{}
	// RawBody is kept so a test can assert on the ABSENCE of a key, which a
	// decoded map cannot distinguish from a null.
	RawBody string
}

// awxServer answers the API-root discovery handshake exactly as a real upstream
// AWX 24.6.1 does, records the first non-discovery request into got, and replies
// with status/body.
func awxServer(t *testing.T, got *capture, status int, body string) *httptest.Server {
	t.Helper()
	awx.ResetAPIRootCacheForTest()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			_, _ = w.Write([]byte(`{"current_version":"/api/v2/","available_versions":{"v2":"/api/v2/"}}`))
			return
		case "/api/v2/me/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"username":"admin"}]}`))
			return
		}

		raw, _ := io.ReadAll(r.Body)
		got.Method = r.Method
		got.Path = r.URL.Path
		got.Query = r.URL.Query()
		got.RawBody = string(raw)
		got.Body = map[string]interface{}{}
		if len(strings.TrimSpace(string(raw))) > 0 {
			_ = json.Unmarshal(raw, &got.Body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func boolean(name string, value interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: value}
}

func text(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: value}
}

// auth is the credential block every test prepends. The token is a realistic
// 30-character PAT, not a toy: awx.Redact does a plain substring replace, so a
// short secret would corrupt the very error strings some of these tests assert on.
func auth(base string) []*core.Connection {
	return []*core.Connection{
		str("awx_url", base),
		str("auth_method", "token"),
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}
}

func with(base string, extra ...*core.Connection) []*core.Connection {
	return append(auth(base), extra...)
}

// mustSucceed asserts the SOFT-failure contract's happy side: no Go error (a Go
// error aborts the whole flow) and success=true.
func mustSucceed(t *testing.T, out map[string]interface{}, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Execute returned a HARD error (that aborts the whole flow): %v", err)
	}
	if out["success"] != true {
		t.Fatalf("expected success, got success=%v error=%v", out["success"], out["error"])
	}
}

// mustSoftFail asserts the other side of the contract: a failure is reported in
// the outputs with a NIL Go error, so the flow keeps walking.
func mustSoftFail(t *testing.T, out map[string]interface{}, err error, wantSubstring string) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected a SOFT failure (nil error), got a hard error: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("expected success=false, got %v", out["success"])
	}
	msg, _ := out["error"].(string)
	if !strings.Contains(msg, wantSubstring) {
		t.Fatalf("error %q does not mention %q", msg, wantSubstring)
	}
}

// ---------------------------------------------------------------------------
// ★ host_group_assign — the disassociate trap
// ---------------------------------------------------------------------------

// TestAddNeverSendsTheDisassociateKey is the most important test in this family.
//
// AWX's attach handler tests `if 'disassociate' in request.data` — the PRESENCE
// of the key, not its value — so {"id":5,"disassociate":false} REMOVES the host.
// A decoded map cannot tell a false from an absent key, so this asserts on the
// RAW BODY.
func TestAddNeverSendsTheDisassociateKey(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusNoContent, "") // attach answers 204, empty body
	defer srv.Close()

	out, err := host_group_assign.Execute(nil, nil, with(srv.URL,
		str("inventory_id", "1"),
		str("group_id", "9"),
		str("host_id", "5"),
		str("operation", "add"),
	))
	mustSucceed(t, out, err)

	if strings.Contains(got.RawBody, "disassociate") {
		t.Fatalf("★ THE DISASSOCIATE TRAP: an ADD put the disassociate key on the wire (%s). "+
			"AWX checks for the KEY, not its value — this would have REMOVED the host.", got.RawBody)
	}
	if _, present := got.Body["disassociate"]; present {
		t.Fatalf("disassociate key present in an add body: %v", got.Body)
	}
	if got.Body["id"] != float64(5) {
		t.Errorf("body id = %v, want 5", got.Body["id"])
	}
	if out["operation"] != "add" || out["host_id"] != "5" || out["group_id"] != "9" {
		t.Errorf("unexpected outputs: %v", out)
	}
}

// TestUsesTheGroupsHostsRelationNotTheInventorySublist pins the OTHER trap:
// /inventories/{id}/hosts/ is a parent_key sublist where a disassociate
// HARD-DELETES the host. Only /groups/{id}/hosts/ is safe.
func TestUsesTheGroupsHostsRelationNotTheInventorySublist(t *testing.T) {
	for _, operation := range []string{"add", "remove"} {
		got := &capture{}
		srv := awxServer(t, got, http.StatusNoContent, "")

		out, err := host_group_assign.Execute(nil, nil, with(srv.URL,
			str("inventory_id", "1"),
			str("group_id", "9"),
			str("host_id", "5"),
			str("operation", operation),
		))
		mustSucceed(t, out, err)
		srv.Close()

		if got.Path != "/api/v2/groups/9/hosts/" {
			t.Fatalf("%s posted to %q — MUST be /api/v2/groups/9/hosts/. On the inventory sublist a "+
				"disassociate hard-deletes the host.", operation, got.Path)
		}
		if got.Method != http.MethodPost {
			t.Errorf("%s used %s, want POST", operation, got.Method)
		}
	}
}

// TestRemoveSendsDisassociateTrue is the positive half of the trap.
func TestRemoveSendsDisassociateTrue(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusNoContent, "")
	defer srv.Close()

	out, err := host_group_assign.Execute(nil, nil, with(srv.URL,
		str("inventory_id", "1"),
		str("group_id", "9"),
		str("host_id", "5"),
		str("operation", "remove"),
	))
	mustSucceed(t, out, err)

	if got.Body["disassociate"] != true {
		t.Fatalf("remove did not send disassociate:true — body was %s", got.RawBody)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "still exists") {
		t.Errorf("the removal summary should reassure the operator the host survives, got %q", summary)
	}
}

func TestGroupAssignRejectsAnUnknownOperation(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusNoContent, "")
	defer srv.Close()

	out, err := host_group_assign.Execute(nil, nil, with(srv.URL,
		str("inventory_id", "1"),
		str("group_id", "9"),
		str("host_id", "5"),
		str("operation", "delete"), // not one of the two options
	))
	mustSoftFail(t, out, err, "Operation must be")

	if got.Method != "" {
		t.Fatalf("an invalid operation still called AWX (%s %s)", got.Method, got.Path)
	}
}

// TestGroupAssignMissingHostIsASoftFailure pins the failure contract: a missing
// required field must NOT abort the flow.
func TestGroupAssignMissingHostIsASoftFailure(t *testing.T) {
	srv := awxServer(t, &capture{}, http.StatusNoContent, "")
	defer srv.Close()

	out, err := host_group_assign.Execute(nil, nil, with(srv.URL,
		str("inventory_id", "1"),
		str("group_id", "9"),
		str("operation", "add"),
	))
	mustSoftFail(t, out, err, "Host is required")
}

// TestMissingCredentialIsAHardError pins the one case that IS a hard error.
func TestMissingCredentialIsAHardError(t *testing.T) {
	out, err := host_group_assign.Execute(nil, nil, []*core.Connection{
		str("awx_url", "https://awx.example.com"),
		str("auth_method", "token"),
		// no api_token
	})
	if err == nil {
		t.Fatalf("a missing credential must be a HARD error; got outputs %v", out)
	}
}

// ---------------------------------------------------------------------------
// host_list
// ---------------------------------------------------------------------------

func TestHostListFiltersAndPages(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusOK,
		`{"count":3,"next":"/api/v2/hosts/?page=2","previous":null,"results":[{"id":5,"name":"web01"},{"id":6,"name":"web02"}]}`)
	defer srv.Close()

	out, err := host_list.Execute(nil, nil, with(srv.URL,
		str("inventory_id", "1"),
		str("group_id", "9"),
		str("search", "web"),
		str("name", "web01"),
		str("enabled", "true"),
		&core.Connection{Name: "page_size", Type: core.ConnectionTypeInteger, Value: 2},
	))
	mustSucceed(t, out, err)

	if got.Path != "/api/v2/hosts/" {
		t.Fatalf("listed %q, want /api/v2/hosts/", got.Path)
	}
	// group_id maps to the reverse relation, NOT to a sublist endpoint.
	for param, want := range map[string]string{
		"inventory":  "1",
		"groups__id": "9",
		"search":     "web",
		"name":       "web01",
		"enabled":    "true",
		"page_size":  "2",
		"order_by":   "name", // the Go-side default: the manifest cannot carry one
	} {
		if v := got.Query[param]; len(v) != 1 || v[0] != want {
			t.Errorf("query %s = %v, want [%s]", param, v, want)
		}
	}

	if out["count"] != 2 || out["total_count"] != 3 || out["has_more"] != true {
		t.Errorf("count=%v total_count=%v has_more=%v, want 2/3/true", out["count"], out["total_count"], out["has_more"])
	}
	if results, ok := out["results"].([]interface{}); !ok || len(results) != 2 {
		t.Errorf("results = %v", out["results"])
	}
}

// TestHostListClampsAnOversizedPageSize — AWX clamps to 200 SILENTLY, so a caller
// that trusted its own page_size would quietly miss rows. We clamp visibly.
func TestHostListClampsAnOversizedPageSize(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusOK, `{"count":0,"next":null,"results":[]}`)
	defer srv.Close()

	out, err := host_list.Execute(nil, nil, with(srv.URL,
		&core.Connection{Name: "page_size", Type: core.ConnectionTypeInteger, Value: 5000},
	))
	mustSucceed(t, out, err)

	if v := got.Query["page_size"]; len(v) != 1 || v[0] != "200" {
		t.Errorf("page_size = %v, want [200] (AWX's MAX_PAGE_SIZE)", v)
	}
}

// ---------------------------------------------------------------------------
// host_get
// ---------------------------------------------------------------------------

func TestHostGetFlattensTheHost(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusOK, `{
		"id":5,"name":"web01","enabled":true,"has_active_failures":true,
		"last_job":77,
		"summary_fields":{"last_job":{"id":77,"name":"Deploy","status":"failed"}}
	}`)
	defer srv.Close()

	out, err := host_get.Execute(nil, nil, with(srv.URL, str("inventory_id", "1"), str("host_id", "5")))
	mustSucceed(t, out, err)

	if got.Path != "/api/v2/hosts/5/" {
		t.Fatalf("fetched %q, want /api/v2/hosts/5/", got.Path)
	}
	if out["id"] != "5" || out["name"] != "web01" {
		t.Errorf("id=%v name=%v", out["id"], out["name"])
	}
	if out["enabled"] != true || out["has_active_failures"] != true {
		t.Errorf("enabled=%v has_active_failures=%v", out["enabled"], out["has_active_failures"])
	}
	job, ok := out["last_job"].(map[string]interface{})
	if !ok || job["status"] != "failed" {
		t.Errorf("last_job = %v, want the rich summary_fields object", out["last_job"])
	}
}

// TestHostGet404IsASoftFailureThatDoesNotSayDeleted — AWX hides objects you
// cannot SEE behind a 404, so a 404 must never be reported as "deleted", and it
// must not abort the flow.
func TestHostGet404IsASoftFailure(t *testing.T) {
	srv := awxServer(t, &capture{}, http.StatusNotFound, `{"detail":"Not found."}`)
	defer srv.Close()

	out, err := host_get.Execute(nil, nil, with(srv.URL, str("host_id", "404")))
	mustSoftFail(t, out, err, "permission")
	if msg, _ := out["error"].(string); strings.Contains(strings.ToLower(msg), "deleted") {
		t.Errorf("a 404 must not be reported as 'deleted': %q", msg)
	}
}

// ---------------------------------------------------------------------------
// host_create — the `enabled` tri-state
// ---------------------------------------------------------------------------

func TestHostCreatePostsTheHost(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusCreated, `{"id":5,"name":"web01.example.com","enabled":true}`)
	defer srv.Close()

	out, err := host_create.Execute(nil, nil, with(srv.URL,
		str("inventory_id", "1"),
		str("name", "web01.example.com"),
		str("description", "front end"),
		str("instance_id", "i-0abc123"),
		text("variables", "ansible_host: 10.0.0.5"),
		&core.Connection{Name: "additional_fields", Type: core.ConnectionTypeObject, Value: `{"instance_id":"i-override"}`},
	))
	mustSucceed(t, out, err)

	if got.Method != http.MethodPost || got.Path != "/api/v2/hosts/" {
		t.Fatalf("%s %s, want POST /api/v2/hosts/", got.Method, got.Path)
	}
	// The id must be a JSON NUMBER — AWX rejects a string for a foreign key.
	if got.Body["inventory"] != float64(1) {
		t.Errorf("inventory = %#v, want the JSON number 1", got.Body["inventory"])
	}
	if got.Body["name"] != "web01.example.com" || got.Body["description"] != "front end" {
		t.Errorf("body = %s", got.RawBody)
	}
	// Variables travel as a YAML-or-JSON STRING, not as a nested object.
	if got.Body["variables"] != "ansible_host: 10.0.0.5" {
		t.Errorf("variables = %#v, want the raw string", got.Body["variables"])
	}
	// Additional Fields is the power-user's LAST WORD.
	if got.Body["instance_id"] != "i-override" {
		t.Errorf("additional_fields did not override instance_id: %s", got.RawBody)
	}
	if out["id"] != "5" {
		t.Errorf("id = %v, want 5", out["id"])
	}
}

// TestHostCreateOmitsEnabledWhenUntouched is the tri-state guard. AWX defaults
// Host.enabled to TRUE and the manifest cannot carry a default, so the checkbox
// renders unticked. Sending enabled:false for an untouched box would silently
// create every host DISABLED — invisible until a playbook ran against nothing.
func TestHostCreateOmitsEnabledWhenUntouched(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusCreated, `{"id":5,"name":"web01"}`)
	defer srv.Close()

	out, err := host_create.Execute(nil, nil, with(srv.URL, str("inventory_id", "1"), str("name", "web01")))
	mustSucceed(t, out, err)

	if _, present := got.Body["enabled"]; present {
		t.Fatalf("an untouched Enabled checkbox sent enabled=%v — it must be OMITTED so AWX applies "+
			"its own default of true. Body: %s", got.Body["enabled"], got.RawBody)
	}
}

// …and when the operator DOES set it, it is sent — including the false a bound
// ${var.x} arrives as (the flow engine substitutes every reference to a string).
func TestHostCreateSendsEnabledWhenSet(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value interface{}
		want  bool
	}{
		{"ticked", true, true},
		{"bound-variable-true", "true", true},
		{"bound-variable-false", "false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := &capture{}
			srv := awxServer(t, got, http.StatusCreated, `{"id":5}`)
			defer srv.Close()

			out, err := host_create.Execute(nil, nil, with(srv.URL,
				str("inventory_id", "1"), str("name", "web01"), boolean("enabled", tc.value)))
			mustSucceed(t, out, err)

			if got.Body["enabled"] != tc.want {
				t.Fatalf("enabled = %#v, want %v. Body: %s", got.Body["enabled"], tc.want, got.RawBody)
			}
		})
	}
}

func TestHostCreateMissingNameIsASoftFailure(t *testing.T) {
	srv := awxServer(t, &capture{}, http.StatusCreated, `{"id":5}`)
	defer srv.Close()

	out, err := host_create.Execute(nil, nil, with(srv.URL, str("inventory_id", "1")))
	mustSoftFail(t, out, err, "Host Name is required")
}

// ---------------------------------------------------------------------------
// host_update
// ---------------------------------------------------------------------------

// TestHostUpdateNeverSendsTheInventory — Host.inventory is READ-ONLY once the
// host exists. AWX takes the PATCH, answers 200 and changes nothing, so a node
// that sent it would report a successful "move" that never happened.
func TestHostUpdateNeverSendsTheInventory(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusOK, `{"id":5,"name":"web01-renamed"}`)
	defer srv.Close()

	out, err := host_update.Execute(nil, nil, with(srv.URL,
		str("inventory_id", "2"), // a DIFFERENT inventory — must not be acted on
		str("host_id", "5"),
		str("name", "web01-renamed"),
	))
	mustSucceed(t, out, err)

	if got.Method != http.MethodPatch {
		t.Fatalf("used %s — AWX updates MUST be PATCH; a PUT resets every omitted field to its model default", got.Method)
	}
	if got.Path != "/api/v2/hosts/5/" {
		t.Fatalf("patched %q, want /api/v2/hosts/5/", got.Path)
	}
	if _, present := got.Body["inventory"]; present {
		t.Fatalf("the PATCH carried an inventory (%v) — it is read-only and would be silently ignored, "+
			"making a failed move look like a success. Body: %s", got.Body["inventory"], got.RawBody)
	}
	if got.Body["name"] != "web01-renamed" {
		t.Errorf("body = %s", got.RawBody)
	}
}

func TestHostUpdateOmitsUntouchedFields(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusOK, `{"id":5,"name":"web01"}`)
	defer srv.Close()

	out, err := host_update.Execute(nil, nil, with(srv.URL,
		str("host_id", "5"),
		str("description", "moved to rack 4"),
		str("name", ""), // blank = leave alone, NOT blank it out
	))
	mustSucceed(t, out, err)

	if _, present := got.Body["name"]; present {
		t.Errorf("a blank Name was sent as %#v — it must be omitted, not blanked", got.Body["name"])
	}
	if _, present := got.Body["enabled"]; present {
		t.Errorf("an untouched Enabled checkbox was sent as %#v — it must be omitted", got.Body["enabled"])
	}
	if got.Body["description"] != "moved to rack 4" {
		t.Errorf("body = %s", got.RawBody)
	}
}

func TestHostUpdateWithNothingToChangeIsASoftFailure(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusOK, `{"id":5}`)
	defer srv.Close()

	out, err := host_update.Execute(nil, nil, with(srv.URL, str("inventory_id", "1"), str("host_id", "5")))
	mustSoftFail(t, out, err, "nothing to update")

	if got.Method != "" {
		t.Fatalf("an empty update still PATCHed AWX (%s %s)", got.Method, got.Path)
	}
}

// ---------------------------------------------------------------------------
// host_delete
// ---------------------------------------------------------------------------

func TestHostDeleteRequiresConfirmation(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusNoContent, "")
	defer srv.Close()

	out, err := host_delete.Execute(nil, nil, with(srv.URL, str("inventory_id", "1"), str("host_id", "5")))
	mustSoftFail(t, out, err, "Confirm Destructive Action")

	if got.Method != "" {
		t.Fatalf("★ an unconfirmed delete still called AWX (%s %s)", got.Method, got.Path)
	}
}

// The guard fails CLOSED: an unresolvable ${var.x} substitutes to the empty
// string, which must decline to delete rather than delete.
func TestHostDeleteFailsClosedOnAnUnresolvedVariable(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusNoContent, "")
	defer srv.Close()

	out, err := host_delete.Execute(nil, nil, with(srv.URL,
		str("host_id", "5"), boolean("confirm_destructive", "")))
	mustSoftFail(t, out, err, "Confirm Destructive Action")

	if got.Method != "" {
		t.Fatalf("★ a blank confirmation still deleted host 5 (%s %s)", got.Method, got.Path)
	}
}

func TestHostDeleteConfirmed(t *testing.T) {
	got := &capture{}
	srv := awxServer(t, got, http.StatusNoContent, "") // AWX answers 204, empty body
	defer srv.Close()

	out, err := host_delete.Execute(nil, nil, with(srv.URL,
		str("inventory_id", "1"), str("host_id", "5"), boolean("confirm_destructive", true)))
	mustSucceed(t, out, err)

	if got.Method != http.MethodDelete || got.Path != "/api/v2/hosts/5/" {
		t.Fatalf("%s %s, want DELETE /api/v2/hosts/5/", got.Method, got.Path)
	}
	if out["id"] != "5" || out["deleted"] != true {
		t.Errorf("id=%v deleted=%v", out["id"], out["deleted"])
	}
}

// A 409 means a job is still running against the host — a retryable condition,
// and a SOFT failure, not a flow-aborting one.
func TestHostDeleteActiveJobsIsASoftFailure(t *testing.T) {
	srv := awxServer(t, &capture{}, http.StatusConflict,
		`{"error":"Resource is being used by running jobs.","active_jobs":[{"type":"job","id":12}]}`)
	defer srv.Close()

	out, err := host_delete.Execute(nil, nil, with(srv.URL,
		str("host_id", "5"), boolean("confirm_destructive", true)))
	mustSoftFail(t, out, err, "still running")
}
