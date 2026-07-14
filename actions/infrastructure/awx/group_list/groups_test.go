// Tests for the five AWX group actions (list / get / create / update / delete).
//
// They live in one file because the five share every invariant that is worth
// pinning — the soft-failure contract, the trailing slash, the read-only
// `inventory` field on update — and because the family's real hazard is
// cross-cutting: group_delete is recursive, so its guard must hold before a single
// byte reaches AWX.
package infrastructure_awx_group_list

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
	group_create "flomation.app/automate/executor/actions/infrastructure/awx/group_create"
	group_delete "flomation.app/automate/executor/actions/infrastructure/awx/group_delete"
	group_get "flomation.app/automate/executor/actions/infrastructure/awx/group_get"
	group_update "flomation.app/automate/executor/actions/infrastructure/awx/group_update"

	. "github.com/onsi/gomega"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

func str(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

func boolean(name string, val interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: val}
}

// creds is the auth block every test starts from. The token is a realistic
// 30-character PAT, not a toy: awx.Redact substring-replaces the secret out of
// every message, so a short one would corrupt the assertions themselves.
func creds(base string) []*core.Connection {
	return []*core.Connection{
		str("awx_url", base),
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}
}

// awxServer answers the API-root discovery handshake exactly as a real upstream
// AWX 24.6.1 does, and delegates everything else to h.
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

func bodyOf(r *http.Request) map[string]interface{} {
	raw, _ := io.ReadAll(r.Body)
	out := map[string]interface{}{}
	_ = json.Unmarshal(raw, &out)
	return out
}

// ---------------------------------------------------------------------------
// group_list
// ---------------------------------------------------------------------------

func TestGroupListHappyPath(t *testing.T) {
	RegisterTestingT(t)

	var gotQuery string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/groups/"))
		Expect(r.Method).To(Equal(http.MethodGet))
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"count":2,"next":null,"results":[
			{"id":11,"name":"webservers"},
			{"id":12,"name":"db"}]}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, append(creds(srv.URL),
		str("inventory_id", "5"),
		str("search", "web"),
	))

	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["count"]).To(Equal(2))
	Expect(out["total_count"]).To(Equal(2))
	Expect(out["has_more"]).To(BeFalse())
	Expect(out["results"]).To(HaveLen(2))

	// The inventory is a FILTER (?inventory=), not a path segment, and the list
	// defaults to name order rather than AWX's id-ascending creation order.
	Expect(gotQuery).To(ContainSubstring("inventory=5"))
	Expect(gotQuery).To(ContainSubstring("search=web"))
	Expect(gotQuery).To(ContainSubstring("order_by=name"))
	Expect(gotQuery).To(ContainSubstring("page_size=50"))
}

// A 404 — which is also how AWX hides an object you may not see — must be a SOFT
// failure. A non-nil Go error would abort the whole flow run.
func TestGroupListSoftFailsOnAPIError(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not found."}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, append(creds(srv.URL), str("inventory_id", "5")))

	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("404"))
}

// A missing credential is the ONE hard error: the node is mis-configured, not the
// request.
func TestGroupListHardFailsWithoutACredential(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, []*core.Connection{str("awx_url", "https://awx.example.com")})

	Expect(err).To(HaveOccurred())
	Expect(out).To(BeNil())
}

// ---------------------------------------------------------------------------
// group_get
// ---------------------------------------------------------------------------

func TestGroupGetHappyPath(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/groups/11/"))
		_, _ = w.Write([]byte(`{"id":11,"name":"webservers","variables":"http_port: 8080"}`))
	})
	defer srv.Close()

	out, err := group_get.Execute(nil, nil, append(creds(srv.URL),
		str("inventory_id", "5"),
		str("group_id", "11"),
	))

	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["id"]).To(Equal("11"))
	Expect(out["name"]).To(Equal("webservers"))
	Expect(out["result"]).To(HaveKeyWithValue("variables", "http_port: 8080"))
}

func TestGroupGetRequiresAGroupSoftly(t *testing.T) {
	RegisterTestingT(t)

	out, err := group_get.Execute(nil, nil, append(creds("https://awx.example.com"), str("inventory_id", "5")))

	Expect(err).ToNot(HaveOccurred()) // soft, not hard
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Group is required"))
}

// ---------------------------------------------------------------------------
// group_create
// ---------------------------------------------------------------------------

func TestGroupCreateHappyPath(t *testing.T) {
	RegisterTestingT(t)

	var got map[string]interface{}
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		// The trailing slash is load-bearing: Django APPEND_SLASH 301s a slash-less
		// POST and Go re-issues it as a bodyless GET.
		Expect(r.URL.Path).To(Equal("/api/v2/groups/"))
		Expect(r.Method).To(Equal(http.MethodPost))
		got = bodyOf(r)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":21,"name":"webservers","inventory":5}`))
	})
	defer srv.Close()

	out, err := group_create.Execute(nil, nil, append(creds(srv.URL),
		str("inventory_id", "5"),
		str("name", "webservers"),
		str("description", "the web tier"),
		&core.Connection{Name: "variables", Type: core.ConnectionTypeText, Value: "http_port: 8080"},
	))

	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["id"]).To(Equal("21"))

	// inventory must be a JSON NUMBER, not the string the live dropdown wrote.
	Expect(got["inventory"]).To(BeEquivalentTo(5))
	Expect(got["name"]).To(Equal("webservers"))
	Expect(got["description"]).To(Equal("the web tier"))
	// Group variables are a YAML/JSON *string* to AWX, passed straight through.
	Expect(got["variables"]).To(Equal("http_port: 8080"))
}

// AWX's two unguessable 400s are re-worded rather than passed through raw.
func TestGroupCreateRewordsAWXsRejections(t *testing.T) {
	RegisterTestingT(t)

	cases := []struct {
		awxSays string
		expect  string
	}{
		{`{"name":["Invalid group name."]}`, "reserved by Ansible"},
		{`{"name":["A Host with that name already exists."]}`, "does not allow a group and a host to share a name"},
	}

	for _, tc := range cases {
		body := tc.awxSays
		srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(body))
		})

		out, err := group_create.Execute(nil, nil, append(creds(srv.URL),
			str("inventory_id", "5"),
			str("name", "all"),
		))
		srv.Close()

		Expect(err).ToNot(HaveOccurred())
		Expect(out["success"]).To(BeFalse())
		Expect(out["error"]).To(ContainSubstring(tc.expect))
	}
}

// ---------------------------------------------------------------------------
// group_update
// ---------------------------------------------------------------------------

// PATCH, never PUT — and `inventory` is read-only after creation, so it must not
// be sent even though the node carries an Inventory input (which exists only to
// scope the Group picker).
func TestGroupUpdatePatchesAndNeverSendsInventory(t *testing.T) {
	RegisterTestingT(t)

	var got map[string]interface{}
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/api/v2/groups/11/"))
		Expect(r.Method).To(Equal(http.MethodPatch))
		got = bodyOf(r)
		_, _ = w.Write([]byte(`{"id":11,"name":"web-tier"}`))
	})
	defer srv.Close()

	out, err := group_update.Execute(nil, nil, append(creds(srv.URL),
		str("inventory_id", "5"),
		str("group_id", "11"),
		str("name", "web-tier"),
	))

	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["id"]).To(Equal("11"))
	Expect(got).To(HaveKeyWithValue("name", "web-tier"))
	Expect(got).ToNot(HaveKey("inventory"))
	// An untouched field is omitted, never blanked — a PATCH carrying
	// "variables":"" would wipe the group's variables.
	Expect(got).ToNot(HaveKey("variables"))
}

func TestGroupUpdateRefusesAnEmptyPatch(t *testing.T) {
	RegisterTestingT(t)

	var hits int32
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"id":11}`))
	})
	defer srv.Close()

	out, err := group_update.Execute(nil, nil, append(creds(srv.URL), str("group_id", "11")))

	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Nothing to update"))
	Expect(atomic.LoadInt32(&hits)).To(BeZero()) // and it never touched AWX
}

// ---------------------------------------------------------------------------
// group_delete — ★ the family's trap
// ---------------------------------------------------------------------------

// THE TRAP. GroupDetail.destroy calls delete_recursive(): it removes the group,
// every descendant group left parentless, AND EVERY HOST LEFT IN NO GROUP AT ALL.
// So the guard must fail closed and no request may be made — an unticked box is
// the difference between un-filing machines and destroying them.
func TestGroupDeleteRefusesWithoutConfirmationAndNeverCallsAWX(t *testing.T) {
	RegisterTestingT(t)

	var hits int32
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	for _, unconfirmed := range []*core.Connection{
		boolean("confirm_destructive", false),
		boolean("confirm_destructive", ""),  // an unresolved ${var.x} substitutes to ""
		str("confirm_destructive", "maybe"), // unparseable
	} {
		out, err := group_delete.Execute(nil, nil, append(creds(srv.URL),
			str("group_id", "11"),
			unconfirmed,
		))

		Expect(err).ToNot(HaveOccurred()) // soft — the flow keeps walking
		Expect(out["success"]).To(BeFalse())
		Expect(out["error"]).To(ContainSubstring("Confirm Destructive Action"))
		Expect(out["deleted"]).To(BeNil())
	}

	// A missing input is a refusal too.
	out, err := group_delete.Execute(nil, nil, append(creds(srv.URL), str("group_id", "11")))
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())

	Expect(atomic.LoadInt32(&hits)).To(BeZero())
}

func TestGroupDeleteHappyPath(t *testing.T) {
	RegisterTestingT(t)

	var method, path string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent) // AWX's 204: no body to parse
	})
	defer srv.Close()

	out, err := group_delete.Execute(nil, nil, append(creds(srv.URL),
		str("inventory_id", "5"),
		str("group_id", "11"),
		boolean("confirm_destructive", true),
	))

	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeTrue())
	Expect(out["deleted"]).To(BeTrue())
	Expect(out["id"]).To(Equal("11"))
	Expect(method).To(Equal(http.MethodDelete))
	Expect(path).To(Equal("/api/v2/groups/11/"))

	// The operator must be told what a "group delete" actually did.
	summary := out["tool_result"].(string)
	Expect(strings.ToLower(summary)).To(ContainSubstring("host"))
	Expect(strings.ToLower(summary)).To(ContainSubstring("child group"))
}

// A 409 means a job is still running against the inventory — retryable, and a soft
// failure that names the running job rather than a bare status code.
func TestGroupDeleteSoftFailsOnActiveJobs(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"Resource is being used by running jobs.","active_jobs":[{"type":"job","id":42}]}`))
	})
	defer srv.Close()

	out, err := group_delete.Execute(nil, nil, append(creds(srv.URL),
		str("group_id", "11"),
		boolean("confirm_destructive", true),
	))

	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("job 42"))
	Expect(out["deleted"]).To(BeNil())
}
