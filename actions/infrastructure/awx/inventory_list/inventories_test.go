// Shared tests for the five AWX inventory actions. They live in one file (rather
// than one per action package) because they share a fake AWX and because the
// interesting behaviour is comparative: what create sends versus what update
// sends, and how delete's asynchronous 202 differs from every other delete in the
// node.
//
// Each test drives the real Execute end to end against an httptest server that
// answers the API-root discovery probe exactly as AWX 24.6.1 does.
package infrastructure_awx_inventory_list_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	inventorycreate "flomation.app/automate/executor/actions/infrastructure/awx/inventory_create"
	inventorydelete "flomation.app/automate/executor/actions/infrastructure/awx/inventory_delete"
	inventoryget "flomation.app/automate/executor/actions/infrastructure/awx/inventory_get"
	inventorylist "flomation.app/automate/executor/actions/infrastructure/awx/inventory_list"
	inventoryupdate "flomation.app/automate/executor/actions/infrastructure/awx/inventory_update"

	. "github.com/onsi/gomega"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

func str(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

func text(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: val}
}

func boolean(name string, val bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: val}
}

func object(name, jsonVal string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: jsonVal}
}

// auth is the credential block every test starts from. The token is a realistic
// 30-character AWX PAT rather than a toy, because awx.Redact scrubs a credential
// with a plain substring replace and a degenerate secret would corrupt the very
// error strings some of these tests assert on.
func auth(base string, rest ...*core.Connection) []*core.Connection {
	in := []*core.Connection{
		str("awx_url", base),
		str("auth_method", "token"),
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}
	return append(in, rest...)
}

// awxServer answers the two root-discovery requests as a real AWX does and hands
// everything else to h.
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

// deadServer fails the test if it is called at all — for the guards that must
// refuse BEFORE any request reaches AWX.
func deadServer(t *testing.T) *httptest.Server {
	awx.ResetAPIRootCacheForTest()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("AWX was called (%s %s) — this action should have refused before making any request", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
}

func decodeBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	Expect(err).To(BeNil())
	body := map[string]interface{}{}
	Expect(json.Unmarshal(raw, &body)).To(Succeed())
	return body
}

// ---------------------------------------------------------------------------
// inventory_list
// ---------------------------------------------------------------------------

func TestInventoryListHappyPath(t *testing.T) {
	RegisterTestingT(t)

	var got *http.Request
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		got = r
		Expect(r.URL.Path).To(Equal("/api/v2/inventories/"))
		_, _ = w.Write([]byte(`{"count":3,"next":"/api/v2/inventories/?page=2","results":[
			{"id":1,"name":"Production","total_hosts":9},
			{"id":2,"name":"Staging","total_hosts":2}]}`))
	})
	defer srv.Close()

	out, err := inventorylist.Execute(nil, nil, auth(srv.URL,
		str("search", "prod"),
		str("organization_id", "3"),
		str("kind", "smart"),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())

	// Defaults live in Go, not in the manifest — assert they actually reach AWX.
	q := got.URL.Query()
	Expect(q.Get("page_size")).To(Equal("50"))
	Expect(q.Get("order_by")).To(Equal("name"))
	Expect(q.Get("search")).To(Equal("prod"))
	Expect(q.Get("organization")).To(Equal("3"))
	Expect(q.Get("kind")).To(Equal("smart"))

	// count is what came back; total_count is AWX's server-side total. They are
	// different numbers here on purpose.
	Expect(out["count"]).To(Equal(2))
	Expect(out["total_count"]).To(Equal(3))
	Expect(out["has_more"]).To(BeTrue())
	Expect(out["results"]).To(HaveLen(2))
}

// A STANDARD inventory's kind is the EMPTY STRING in AWX, so "any kind" and "only
// standard" would be the same value on the wire. The dropdown uses the sentinel
// "standard", which must arrive at AWX as a PRESENT, EMPTY kind parameter — and
// an untouched dropdown must send no kind at all.
func TestInventoryListKindStandardSendsAnEmptyKindFilter(t *testing.T) {
	RegisterTestingT(t)

	var got *http.Request
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		got = r
		_, _ = w.Write([]byte(`{"count":0,"results":[]}`))
	})
	defer srv.Close()

	_, err := inventorylist.Execute(nil, nil, auth(srv.URL, str("kind", "standard")))
	Expect(err).To(BeNil())
	Expect(got.URL.Query().Has("kind")).To(BeTrue(), "the kind filter must be sent")
	Expect(got.URL.Query().Get("kind")).To(Equal(""), "a standard inventory's kind is the empty string")

	_, err = inventorylist.Execute(nil, nil, auth(srv.URL))
	Expect(err).To(BeNil())
	Expect(got.URL.Query().Has("kind")).To(BeFalse(), "an untouched Kind dropdown must not filter")
}

// An AWX 4xx is a SOFT failure: outputs with success=false and a NIL error. A
// non-nil error would abort the whole flow.
func TestInventoryListAWXErrorIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"You do not have permission to perform this action."}`))
	})
	defer srv.Close()

	out, err := inventorylist.Execute(nil, nil, auth(srv.URL))
	Expect(err).To(BeNil(), "an AWX error must never abort the flow")
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("403"))
}

// A missing credential is the ONE hard failure — the node is mis-configured.
func TestMissingCredentialIsAHardFailure(t *testing.T) {
	RegisterTestingT(t)

	out, err := inventorylist.Execute(nil, nil, []*core.Connection{str("awx_url", "https://awx.example.com")})
	Expect(out).To(BeNil())
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("API Token"))
}

// ---------------------------------------------------------------------------
// inventory_get
// ---------------------------------------------------------------------------

func TestInventoryGetHappyPath(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodGet))
		Expect(r.URL.Path).To(Equal("/api/v2/inventories/5/"))
		_, _ = w.Write([]byte(`{"id":5,"name":"Production","total_hosts":9,"total_groups":2,
			"has_active_failures":true,"pending_deletion":false}`))
	})
	defer srv.Close()

	out, err := inventoryget.Execute(nil, nil, auth(srv.URL, str("inventory_id", "5")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["id"]).To(Equal("5"))
	// AWX's counts arrive as JSON floats; they must be emitted as whole numbers.
	Expect(out["total_hosts"]).To(Equal(int64(9)))
	Expect(out["total_groups"]).To(Equal(int64(2)))
	Expect(out["has_active_failures"]).To(BeTrue())
	Expect(out["pending_deletion"]).To(BeFalse())
	Expect(out["tool_result"]).To(ContainSubstring("Production"))
}

func TestInventoryGetMissingIDIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	srv := deadServer(t)
	defer srv.Close()

	out, err := inventoryget.Execute(nil, nil, auth(srv.URL))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Inventory is required"))
}

// ---------------------------------------------------------------------------
// inventory_create
// ---------------------------------------------------------------------------

// ★ variables MUST go to AWX as a STRING. The model field is a TextField behind a
// CharNullField, so a real JSON object is answered with 400 "Not a valid string."
func TestInventoryCreateSendsVariablesAsAString(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		// Django's APPEND_SLASH 301s a slash-less POST and Go drops the body.
		Expect(r.URL.Path).To(Equal("/api/v2/inventories/"))
		Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))
		body = decodeBody(t, r)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":11,"name":"Web Servers"}`))
	})
	defer srv.Close()

	out, err := inventorycreate.Execute(nil, nil, auth(srv.URL,
		str("name", "Web Servers"),
		str("organization_id", "1"),
		str("description", "the web tier"),
		text("variables", "ansible_user: deploy"),
		boolean("prevent_instance_group_fallback", true),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["id"]).To(Equal("11"))

	Expect(body["name"]).To(Equal("Web Servers"))
	Expect(body["description"]).To(Equal("the web tier"))
	Expect(body["variables"]).To(BeAssignableToTypeOf(""), "variables must be a STRING, not an object")
	Expect(body["variables"]).To(Equal("ansible_user: deploy"))
	// An id must be a real JSON number, not the string the dropdown wrote.
	Expect(body["organization"]).To(Equal(float64(1)))
	Expect(body["prevent_instance_group_fallback"]).To(Equal(true))
	// kind is omitted for a standard inventory rather than sent as "".
	Expect(body).ToNot(HaveKey("kind"))
}

// An untouched checkbox must be OMITTED, not sent as false — the manifest cannot
// carry a default Value, so every checkbox renders unticked.
func TestInventoryCreateOmitsAnUntouchedCheckbox(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		body = decodeBody(t, r)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":12,"name":"Bare"}`))
	})
	defer srv.Close()

	_, err := inventorycreate.Execute(nil, nil, auth(srv.URL, str("name", "Bare"), str("organization_id", "1")))
	Expect(err).To(BeNil())
	Expect(body).ToNot(HaveKey("prevent_instance_group_fallback"))
}

// kind=smart with no host filter is a guaranteed AWX 400. Refuse it locally, with
// a message that names the field on the node — and make no request at all.
func TestInventoryCreateSmartWithoutAHostFilterIsRefused(t *testing.T) {
	RegisterTestingT(t)

	srv := deadServer(t)
	defer srv.Close()

	out, err := inventorycreate.Execute(nil, nil, auth(srv.URL,
		str("name", "Smart"),
		str("organization_id", "1"),
		str("kind", "smart"),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Host Filter"))
}

func TestInventoryCreateAdditionalFieldsWinLast(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		body = decodeBody(t, r)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":13,"name":"Override"}`))
	})
	defer srv.Close()

	_, err := inventorycreate.Execute(nil, nil, auth(srv.URL,
		str("name", "First"),
		str("organization_id", "1"),
		object("additional_fields", `{"name":"Override","variables":"---\nfoo: bar"}`),
	))
	Expect(err).To(BeNil())
	Expect(body["name"]).To(Equal("Override"))
	Expect(body["variables"]).To(Equal("---\nfoo: bar"))
}

// ---------------------------------------------------------------------------
// inventory_update
// ---------------------------------------------------------------------------

// PATCH, never PUT: AWX copies each model default onto the serializer, so a PUT
// that omitted a field would RESET it.
func TestInventoryUpdatePatchesOnlyWhatChanged(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPatch))
		Expect(r.URL.Path).To(Equal("/api/v2/inventories/5/"))
		body = decodeBody(t, r)
		_, _ = w.Write([]byte(`{"id":5,"name":"Renamed"}`))
	})
	defer srv.Close()

	out, err := inventoryupdate.Execute(nil, nil, auth(srv.URL,
		str("inventory_id", "5"),
		str("name", "Renamed"),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["id"]).To(Equal("5"))

	Expect(body).To(HaveKeyWithValue("name", "Renamed"))
	// Everything left blank must be absent, or the PATCH would blank it out.
	Expect(body).ToNot(HaveKey("description"))
	Expect(body).ToNot(HaveKey("variables"))
	Expect(body).ToNot(HaveKey("host_filter"))
	// kind is not an input: AWX 405s any attempt to change it after creation.
	Expect(body).ToNot(HaveKey("kind"))
}

func TestInventoryUpdateWithNothingToChangeIsRefused(t *testing.T) {
	RegisterTestingT(t)

	srv := deadServer(t)
	defer srv.Close()

	out, err := inventoryupdate.Execute(nil, nil, auth(srv.URL, str("inventory_id", "5")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("nothing to update"))
}

// ---------------------------------------------------------------------------
// inventory_delete
// ---------------------------------------------------------------------------

// ★ THE TRAP: AWX deletes an inventory ASYNCHRONOUSLY. The DELETE answers 202,
// not 204, and the row lingers with pending_deletion=true until a background task
// has torn its hosts and groups down.
func TestInventoryDeleteIsAsynchronous(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodDelete))
		Expect(r.URL.Path).To(Equal("/api/v2/inventories/5/"))
		w.WriteHeader(http.StatusAccepted) // 202, NOT 204
	})
	defer srv.Close()

	out, err := inventorydelete.Execute(nil, nil, auth(srv.URL,
		str("inventory_id", "5"),
		boolean("confirm_destructive", true),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue(), "202 is AWX accepting the delete, not refusing it")
	Expect(out["id"]).To(Equal("5"))
	Expect(out["deleted"]).To(BeTrue())
	Expect(out["pending_deletion"]).To(BeTrue())
	Expect(out["tool_result"]).To(ContainSubstring("background"))
}

func TestInventoryDeleteHandlesASynchronous204(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	out, err := inventorydelete.Execute(nil, nil, auth(srv.URL,
		str("inventory_id", "5"),
		boolean("confirm_destructive", true),
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["deleted"]).To(BeTrue())
	Expect(out["pending_deletion"]).To(BeFalse())
}

// The guard fails CLOSED, and it must refuse before AWX is touched at all.
func TestInventoryDeleteWithoutConfirmationIsRefused(t *testing.T) {
	RegisterTestingT(t)

	srv := deadServer(t)
	defer srv.Close()

	out, err := inventorydelete.Execute(nil, nil, auth(srv.URL, str("inventory_id", "5")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Confirm Destructive Action"))

	// An unresolved ${var.x} substitutes to the empty string — which must decline
	// to delete, not delete.
	out, err = inventorydelete.Execute(nil, nil, auth(srv.URL,
		str("inventory_id", "5"),
		&core.Connection{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Value: ""},
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
}

// A 409 means a job is still running against the inventory. That is retryable, and
// the operator needs to be told which job — not handed a bare status code.
func TestInventoryDeleteReportsActiveJobs(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"Resource is being used by running jobs.","active_jobs":[{"type":"job","id":42}]}`))
	})
	defer srv.Close()

	out, err := inventorydelete.Execute(nil, nil, auth(srv.URL,
		str("inventory_id", "5"),
		boolean("confirm_destructive", true),
	))
	Expect(err).To(BeNil(), "a 409 must not abort the flow — it is retryable")
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("still running"))
	Expect(out["error"]).To(ContainSubstring("job 42"))
}
