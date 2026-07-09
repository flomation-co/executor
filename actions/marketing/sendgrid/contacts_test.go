// Tests for the Contacts cluster (contact_upsert, contact_get,
// contact_get_by_email, contact_search, contact_list, contact_count,
// contact_delete, contact_import_status) — the marketing-contacts endpoints
// whose writes are asynchronous 202 {"job_id"} jobs, whose reads cap at a
// 50-row sample, and whose by-email lookup answers 404 for "no match".
package sendgrid_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
	contactcount "flomation.app/automate/executor/actions/marketing/sendgrid/contact_count"
	contactdelete "flomation.app/automate/executor/actions/marketing/sendgrid/contact_delete"
	contactget "flomation.app/automate/executor/actions/marketing/sendgrid/contact_get"
	contactgetbyemail "flomation.app/automate/executor/actions/marketing/sendgrid/contact_get_by_email"
	contactimportstatus "flomation.app/automate/executor/actions/marketing/sendgrid/contact_import_status"
	contactlist "flomation.app/automate/executor/actions/marketing/sendgrid/contact_list"
	contactsearch "flomation.app/automate/executor/actions/marketing/sendgrid/contact_search"
	contactupsert "flomation.app/automate/executor/actions/marketing/sendgrid/contact_upsert"
)

// contactsAuth is the minimal credential pair every contacts action needs.
func contactsAuth(extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{secretConn("api_key", "SG.testkey")}, extra...)
}

func TestContactsUpsert(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"job_id":"job-upsert-1"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactupsert.Execute(nil, nil, contactsAuth(
		strConn("email", "jane@example.com"),
		strConn("first_name", "Jane"),
		strConn("alternate_emails", "j@x.com, jd@y.com"),
		strConn("city", "London"),
		objConn("custom_fields", `{"e1_T":"VIP"}`),
		strConn("list_ids", "list-a, list-b"),
		objConn("additional_fields", `{"external_id":"crm-123"}`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	// Async job semantics: the id output IS the job_id and the summary says so.
	if out["id"] != "job-upsert-1" {
		t.Fatalf("id = %v, want the job_id", out["id"])
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "Get Import Status") {
		t.Fatalf("summary does not point at the status poll: %v", summary)
	}
	if gotMethod != http.MethodPut || gotPath != "/v3/marketing/contacts" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	contacts, _ := gotBody["contacts"].([]interface{})
	if len(contacts) != 1 {
		t.Fatalf("contacts = %v", gotBody["contacts"])
	}
	contact, _ := contacts[0].(map[string]interface{})
	if contact["email"] != "jane@example.com" || contact["first_name"] != "Jane" || contact["city"] != "London" {
		t.Fatalf("contact = %v", contact)
	}
	alternates, _ := contact["alternate_emails"].([]interface{})
	if len(alternates) != 2 || alternates[0] != "j@x.com" || alternates[1] != "jd@y.com" {
		t.Fatalf("alternate_emails = %v", contact["alternate_emails"])
	}
	customFields, _ := contact["custom_fields"].(map[string]interface{})
	if customFields["e1_T"] != "VIP" {
		t.Fatalf("custom_fields = %v", contact["custom_fields"])
	}
	// additional_fields merges into the CONTACT object, not the top level.
	if contact["external_id"] != "crm-123" {
		t.Fatalf("additional_fields not merged into contact: %v", contact)
	}
	if _, ok := gotBody["external_id"]; ok {
		t.Fatalf("additional_fields leaked to the top-level body: %v", gotBody)
	}
	// list_ids sits BESIDE contacts at the top level.
	listIDs, _ := gotBody["list_ids"].([]interface{})
	if len(listIDs) != 2 || listIDs[0] != "list-a" || listIDs[1] != "list-b" {
		t.Fatalf("list_ids = %v", gotBody["list_ids"])
	}
}

func TestContactsUpsertValidation(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"job_id":"job-x"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	// Missing email is a soft failure before any request.
	out, err := contactupsert.Execute(nil, nil, contactsAuth())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "email") {
		t.Fatalf("missing email result = %v", out)
	}

	// custom_fields must be a JSON object keyed by field ID.
	out, err = contactupsert.Execute(nil, nil, contactsAuth(
		strConn("email", "jane@example.com"),
		objConn("custom_fields", `["not","an","object"]`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "custom_fields") {
		t.Fatalf("bad custom_fields result = %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestContactsGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/marketing/contacts/c-123" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"c-123","email":"jane@example.com","first_name":"Jane"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactget.Execute(nil, nil, contactsAuth(strConn("contact_id", "c-123")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "c-123" {
		t.Fatalf("id = %v", out["id"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["email"] != "jane@example.com" {
		t.Fatalf("result = %v", result)
	}
}

func TestContactsGetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"errors":[{"message":"contact not found","field":null}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactget.Execute(nil, nil, contactsAuth(strConn("contact_id", "c-missing")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "404") {
		t.Fatalf("404 result = %v", out)
	}
}

func TestContactsGetByEmail(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v3/marketing/contacts/search/emails" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"result":{"jane@example.com":{"contact":{"id":"c-9","email":"jane@example.com"}}}}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	// Mixed case in → lower-cased on the wire and used as the unwrap key.
	out, err := contactgetbyemail.Execute(nil, nil, contactsAuth(strConn("email", "Jane@Example.COM")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	emails, _ := gotBody["emails"].([]interface{})
	if len(emails) != 1 || emails[0] != "jane@example.com" {
		t.Fatalf("emails sent = %v", gotBody["emails"])
	}
	if out["id"] != "c-9" {
		t.Fatalf("id = %v", out["id"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["email"] != "jane@example.com" {
		t.Fatalf("result not unwrapped from result[email].contact: %v", out["result"])
	}
}

func TestContactsGetByEmailNoMatch(t *testing.T) {
	// A zero-match lookup answers 404 — that is "no contact found", a soft
	// failure, not a raw API error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"errors":[{"message":"contacts not found","field":null}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactgetbyemail.Execute(nil, nil, contactsAuth(strConn("email", "ghost@example.com")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "no contact found for ghost@example.com") {
		t.Fatalf("404 result = %v", out)
	}

	// A 200 whose per-email entry carries an error instead of a contact is
	// the same "no match".
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"ghost@example.com":{"error":"contact not found"}}}`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()

	out, err = contactgetbyemail.Execute(nil, nil, contactsAuth(strConn("email", "ghost@example.com")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "no contact found") {
		t.Fatalf("per-email error result = %v", out)
	}
}

func TestContactsSearch(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v3/marketing/contacts/search" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"result":[{"id":"c-1"},{"id":"c-2"}],"contact_count":120,"_metadata":{"self":"x"}}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	sgql := `email LIKE 'jane%' AND CONTAINS(list_ids, 'uuid-1')`
	out, err := contactsearch.Execute(nil, nil, contactsAuth(textConn("query", sgql)))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	// The SGQL travels raw — never interpolated or rewritten.
	if gotBody["query"] != sgql {
		t.Fatalf("query = %v", gotBody["query"])
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 2 {
		t.Fatalf("results = %v", out["results"])
	}
	// count reports the TOTAL matched, beyond the 50-row sample returned.
	if out["count"] != 120 {
		t.Fatalf("count = %v, want the contact_count total", out["count"])
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "120") || !strings.Contains(summary, "first 2") {
		t.Fatalf("summary = %v", summary)
	}
}

func TestContactsSearchBadQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errors":[{"message":"invalid SGQL","field":"query"}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactsearch.Execute(nil, nil, contactsAuth(textConn("query", "not sgql")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "invalid SGQL") {
		t.Fatalf("bad query result = %v", out)
	}
}

func TestContactsList(t *testing.T) {
	// The endpoint is a 50-row sample of the most recently updated contacts —
	// the action's name and description must never present it as "list all".
	if contactlist.Name != "SendGrid: List Recent Contacts" {
		t.Fatalf("Name = %q", contactlist.Name)
	}
	if !strings.Contains(contactlist.Description, "sample of up to 50 most recently updated") {
		t.Fatalf("Description missing the sample warning: %q", contactlist.Description)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/marketing/contacts" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":[{"id":"c-1","email":"a@x.com"},{"id":"c-2","email":"b@x.com"}],"contact_count":57,"_metadata":{"self":"x"}}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactlist.Execute(nil, nil, contactsAuth())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 2 {
		t.Fatalf("results = %v", out["results"])
	}
	if out["count"] != 57 {
		t.Fatalf("count = %v, want the account total", out["count"])
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "sample") {
		t.Fatalf("summary = %v", summary)
	}
}

func TestContactsListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errors":[{"message":"internal error","field":null}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactlist.Execute(nil, nil, contactsAuth())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "internal error") {
		t.Fatalf("500 result = %v", out)
	}
}

func TestContactsCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/marketing/contacts/count" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"contact_count":1234,"billable_count":1200}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactcount.Execute(nil, nil, contactsAuth())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "" {
		t.Fatalf("id = %v, want empty", out["id"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["contact_count"] != float64(1234) || result["billable_count"] != float64(1200) {
		t.Fatalf("result = %v", result)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "1234") {
		t.Fatalf("summary = %v", summary)
	}
}

func TestContactsCountError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errors":[{"message":"authorization required","field":null}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactcount.Execute(nil, nil, contactsAuth())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "authorization required") {
		t.Fatalf("401 result = %v", out)
	}
}

func TestContactsDeleteByIDs(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v3/marketing/contacts" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		if r.URL.Query().Get("ids") != "c-1,c-2" {
			t.Errorf("ids = %q", r.URL.Query().Get("ids"))
		}
		if r.URL.Query().Get("delete_all_contacts") != "" {
			t.Errorf("delete_all_contacts sent on an ids delete: %q", r.URL.Query().Get("delete_all_contacts"))
		}
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"job_id":"job-del-1"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	// Spaces around the commas are trimmed before the ids hit the query.
	out, err := contactdelete.Execute(nil, nil, contactsAuth(strConn("contact_ids", " c-1 , c-2 ")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v, query %q)", out["success"], out["error"], gotQuery)
	}
	if out["id"] != "job-del-1" {
		t.Fatalf("id = %v, want the job_id", out["id"])
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "Get Import Status") {
		t.Fatalf("summary does not point at the status poll: %v", summary)
	}
}

func TestContactsDeleteAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("delete_all_contacts") != "true" {
			t.Errorf("delete_all_contacts = %q", r.URL.Query().Get("delete_all_contacts"))
		}
		if r.URL.Query().Get("ids") != "" {
			t.Errorf("ids sent on a delete-all: %q", r.URL.Query().Get("ids"))
		}
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"job_id":"job-del-all"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactdelete.Execute(nil, nil, contactsAuth(boolConn("delete_all_contacts", true)))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "job-del-all" {
		t.Fatalf("id = %v", out["id"])
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "ALL contacts") {
		t.Fatalf("summary = %v", summary)
	}
}

func TestContactsDeleteXORValidation(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"job_id":"job-x"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	// Both modes at once is refused.
	out, err := contactdelete.Execute(nil, nil, contactsAuth(
		strConn("contact_ids", "c-1"),
		boolConn("delete_all_contacts", true),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "not both") {
		t.Fatalf("both-modes result = %v", out)
	}

	// Neither mode is refused too.
	out, err = contactdelete.Execute(nil, nil, contactsAuth(boolConn("delete_all_contacts", false)))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "Delete All Contacts") {
		t.Fatalf("neither-mode result = %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite XOR validation failure (%d calls)", calls)
	}
}

func TestContactsImportStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/marketing/contacts/imports/job-upsert-1" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"job-upsert-1","status":"completed","job_type":"upsert","results":{"requested_count":1,"created_count":1,"updated_count":0,"errored_count":0,"errors_url":""},"started_at":"2026-07-09T10:00:00Z","finished_at":"2026-07-09T10:00:05Z"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactimportstatus.Execute(nil, nil, contactsAuth(strConn("job_id", "job-upsert-1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "job-upsert-1" {
		t.Fatalf("id = %v", out["id"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["status"] != "completed" {
		t.Fatalf("result = %v", result)
	}
	counts, _ := result["results"].(map[string]interface{})
	if counts["created_count"] != float64(1) {
		t.Fatalf("results counts = %v", result["results"])
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "completed") {
		t.Fatalf("summary = %v", summary)
	}
}

func TestContactsImportStatusMissingJobID(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := contactimportstatus.Execute(nil, nil, contactsAuth())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "job_id") {
		t.Fatalf("missing job_id result = %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}
