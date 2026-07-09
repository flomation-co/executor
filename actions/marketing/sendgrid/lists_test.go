// Tests for the lists/custom-fields/segments cluster: list_create, list_get,
// list_get_all, list_update, list_delete (200 {job_id} AND 204 empty),
// list_remove_contacts (202 async), custom_field_list (reserved-field merge),
// segment_list and segment_get (the 2.0 paths). Shared input constructors live
// in common_test.go.
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
	customfieldlist "flomation.app/automate/executor/actions/marketing/sendgrid/custom_field_list"
	listcreate "flomation.app/automate/executor/actions/marketing/sendgrid/list_create"
	listdelete "flomation.app/automate/executor/actions/marketing/sendgrid/list_delete"
	listget "flomation.app/automate/executor/actions/marketing/sendgrid/list_get"
	listgetall "flomation.app/automate/executor/actions/marketing/sendgrid/list_get_all"
	listremovecontacts "flomation.app/automate/executor/actions/marketing/sendgrid/list_remove_contacts"
	listupdate "flomation.app/automate/executor/actions/marketing/sendgrid/list_update"
	segmentget "flomation.app/automate/executor/actions/marketing/sendgrid/segment_get"
	segmentlist "flomation.app/automate/executor/actions/marketing/sendgrid/segment_list"
)

// listsInputs assembles the auth pair plus per-test extras.
func listsInputs(extra ...*core.Connection) []*core.Connection {
	base := []*core.Connection{secretConn("api_key", "SG.testkey")}
	return append(base, extra...)
}

func TestListsCreate(t *testing.T) {
	calls := 0
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"lst-uuid-1","name":"Newsletter","contact_count":0}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := listcreate.Execute(nil, nil, listsInputs(strConn("name", "Newsletter")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if gotMethod != http.MethodPost || gotPath != "/v3/marketing/lists" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if gotBody["name"] != "Newsletter" {
		t.Fatalf("body = %v", gotBody)
	}
	if out["id"] != "lst-uuid-1" {
		t.Fatalf("id = %v", out["id"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["name"] != "Newsletter" {
		t.Fatalf("result = %v", result)
	}

	// A missing name is a soft validation failure before any request is made.
	calls = 0
	out, err = listcreate.Execute(nil, nil, listsInputs())
	if err != nil {
		t.Fatalf("Execute missing name: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "name") {
		t.Fatalf("missing name result = %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}

	// A duplicate name surfaces SendGrid's error envelope as a soft failure.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errors":[{"message":"list name already exists","field":"name"}]}`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()
	out, err = listcreate.Execute(nil, nil, listsInputs(strConn("name", "Newsletter")))
	if err != nil {
		t.Fatalf("Execute duplicate: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "already exists") {
		t.Fatalf("duplicate name result = %v", out)
	}
}

func TestListsGet(t *testing.T) {
	calls := 0
	var gotPath, gotSample string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotPath = r.URL.Path
		gotSample = r.URL.Query().Get("contact_sample")
		_, _ = w.Write([]byte(`{"id":"lst-1","name":"Newsletter","contact_count":42,"contact_sample":[{"id":"c1","email":"a@x.com"}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := listget.Execute(nil, nil, listsInputs(strConn("list_id", "lst-1"), boolConn("contact_sample", true)))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if gotPath != "/v3/marketing/lists/lst-1" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotSample != "true" {
		t.Fatalf("contact_sample = %q", gotSample)
	}
	if out["id"] != "lst-1" {
		t.Fatalf("id = %v", out["id"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["contact_count"] != float64(42) {
		t.Fatalf("result = %v", result)
	}
	if !strings.Contains(out["tool_result"].(string), "Newsletter") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}

	// A missing list_id fails soft before any request.
	calls = 0
	out, err = listget.Execute(nil, nil, listsInputs())
	if err != nil {
		t.Fatalf("Execute missing list_id: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "list_id") {
		t.Fatalf("missing list_id result = %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestListsGetAll(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v3/marketing/lists" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("page_token") == "" {
			_, _ = w.Write([]byte(`{"result":[{"id":"l1","name":"A"},{"id":"l2","name":"B"}],"_metadata":{"self":"x","next":"https://api.sendgrid.com/v3/marketing/lists?page_size=1000&page_token=tok2","count":3}}`))
		} else {
			_, _ = w.Write([]byte(`{"result":[{"id":"l3","name":"C"}],"_metadata":{"self":"x","count":3}}`))
		}
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	// return_all follows _metadata.next across pages.
	out, err := listgetall.Execute(nil, nil, listsInputs(boolConn("return_all", true)))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 3 || out["count"] != 3 {
		t.Fatalf("results = %d count = %v", len(results), out["count"])
	}
	if calls != 2 {
		t.Fatalf("expected 2 page fetches, got %d", calls)
	}
	first, _ := results[0].(map[string]interface{})
	if first["id"] != "l1" {
		t.Fatalf("results[0] = %v", results[0])
	}

	// Default mode fetches a single page.
	calls = 0
	out, err = listgetall.Execute(nil, nil, listsInputs())
	if err != nil {
		t.Fatalf("Execute single page: %v", err)
	}
	results, _ = out["results"].([]interface{})
	if len(results) != 2 || calls != 1 {
		t.Fatalf("single page: %d results in %d calls", len(results), calls)
	}

	// An auth failure surfaces as a soft failure.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errors":[{"message":"authorization required","field":null}]}`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()
	out, err = listgetall.Execute(nil, nil, listsInputs())
	if err != nil {
		t.Fatalf("Execute 401: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "authorization required") {
		t.Fatalf("401 result = %v", out)
	}
}

func TestListsUpdate(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"id":"lst-1","name":"Renamed","contact_count":42}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := listupdate.Execute(nil, nil, listsInputs(strConn("list_id", "lst-1"), strConn("name", "Renamed")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if gotMethod != http.MethodPatch || gotPath != "/v3/marketing/lists/lst-1" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if gotBody["name"] != "Renamed" {
		t.Fatalf("body = %v", gotBody)
	}
	if out["id"] != "lst-1" {
		t.Fatalf("id = %v", out["id"])
	}

	// A missing name is a soft validation failure.
	out, err = listupdate.Execute(nil, nil, listsInputs(strConn("list_id", "lst-1")))
	if err != nil {
		t.Fatalf("Execute missing name: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "name") {
		t.Fatalf("missing name result = %v", out)
	}
}

func TestListsDeleteAsyncJob(t *testing.T) {
	var gotMethod, gotPath, gotDeleteContacts string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotDeleteContacts = r.URL.Query().Get("delete_contacts")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"job_id":"job-123"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := listdelete.Execute(nil, nil, listsInputs(strConn("list_id", "lst-1"), boolConn("delete_contacts", true)))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if gotMethod != http.MethodDelete || gotPath != "/v3/marketing/lists/lst-1" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if gotDeleteContacts != "true" {
		t.Fatalf("delete_contacts = %q", gotDeleteContacts)
	}
	if out["id"] != "job-123" {
		t.Fatalf("id = %v", out["id"])
	}
	summary, _ := out["tool_result"].(string)
	if !strings.Contains(summary, "job-123") || !strings.Contains(summary, "processing") {
		t.Fatalf("async summary = %q", summary)
	}
}

func TestListsDeleteNoContent(t *testing.T) {
	var gotDeleteContacts string
	sawQuery := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDeleteContacts = r.URL.Query().Get("delete_contacts")
		sawQuery = true
		w.WriteHeader(204)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := listdelete.Execute(nil, nil, listsInputs(strConn("list_id", "lst-1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if !sawQuery || gotDeleteContacts != "" {
		t.Fatalf("delete_contacts sent unticked: %q", gotDeleteContacts)
	}
	if out["id"] != "lst-1" {
		t.Fatalf("id = %v", out["id"])
	}
	if !strings.Contains(out["tool_result"].(string), "Deleted list lst-1") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}

	// A missing list_id fails soft.
	out, err = listdelete.Execute(nil, nil, listsInputs())
	if err != nil {
		t.Fatalf("Execute missing list_id: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "list_id") {
		t.Fatalf("missing list_id result = %v", out)
	}
}

func TestListsRemoveContacts(t *testing.T) {
	calls := 0
	var gotMethod, gotPath, gotContactIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotMethod, gotPath = r.Method, r.URL.Path
		gotContactIDs = r.URL.Query().Get("contact_ids")
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"job_id":"job-9"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := listremovecontacts.Execute(nil, nil, listsInputs(strConn("list_id", "lst-1"), strConn("contact_ids", "c1, c2")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if gotMethod != http.MethodDelete || gotPath != "/v3/marketing/lists/lst-1/contacts" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if gotContactIDs != "c1,c2" {
		t.Fatalf("contact_ids = %q", gotContactIDs)
	}
	if out["id"] != "job-9" {
		t.Fatalf("id = %v", out["id"])
	}
	summary, _ := out["tool_result"].(string)
	if !strings.Contains(summary, "2 contact(s)") || !strings.Contains(summary, "job-9") {
		t.Fatalf("async summary = %q", summary)
	}

	// Missing contact_ids fails soft before any request.
	calls = 0
	out, err = listremovecontacts.Execute(nil, nil, listsInputs(strConn("list_id", "lst-1")))
	if err != nil {
		t.Fatalf("Execute missing contact_ids: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "contact_ids") {
		t.Fatalf("missing contact_ids result = %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestListsCustomFields(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"custom_fields":[{"id":"e1_T","name":"loyalty_tier","field_type":"Text"}],"reserved_fields":[{"id":"_rf0","name":"first_name","field_type":"Text","read_only":false}],"_metadata":{"self":"x"}}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	// Default: custom fields only, no reserved marker.
	out, err := customfieldlist.Execute(nil, nil, listsInputs())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if gotPath != "/v3/marketing/field_definitions" {
		t.Fatalf("path = %q", gotPath)
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 1 || out["count"] != 1 {
		t.Fatalf("results = %v count = %v", results, out["count"])
	}
	field, _ := results[0].(map[string]interface{})
	if field["id"] != "e1_T" {
		t.Fatalf("results[0] = %v", field)
	}
	if _, marked := field["reserved"]; marked {
		t.Fatalf("custom field carries reserved marker: %v", field)
	}

	// include_reserved merges the reserved fields in with a reserved:true marker.
	out, err = customfieldlist.Execute(nil, nil, listsInputs(boolConn("include_reserved", true)))
	if err != nil {
		t.Fatalf("Execute include_reserved: %v", err)
	}
	results, _ = out["results"].([]interface{})
	if len(results) != 2 || out["count"] != 2 {
		t.Fatalf("results = %v count = %v", results, out["count"])
	}
	reserved, _ := results[1].(map[string]interface{})
	if reserved["name"] != "first_name" || reserved["reserved"] != true {
		t.Fatalf("reserved field not marked: %v", reserved)
	}

	// An API failure surfaces soft.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"errors":[{"message":"access forbidden","field":null}]}`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()
	out, err = customfieldlist.Execute(nil, nil, listsInputs())
	if err != nil {
		t.Fatalf("Execute 403: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "access forbidden") {
		t.Fatalf("403 result = %v", out)
	}
}

func TestListsSegmentList(t *testing.T) {
	var gotPath, gotParentListIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotParentListIDs = r.URL.Query().Get("parent_list_ids")
		_, _ = w.Write([]byte(`{"results":[{"id":"seg-1","name":"Active","contacts_count":10,"query_version":"2"},{"id":"seg-2","name":"Churned","contacts_count":3,"query_version":"2"}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := segmentlist.Execute(nil, nil, listsInputs(strConn("parent_list_ids", "l1, l2")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if gotPath != "/v3/marketing/segments/2.0" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotParentListIDs != "l1,l2" {
		t.Fatalf("parent_list_ids = %q", gotParentListIDs)
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 2 || out["count"] != 2 {
		t.Fatalf("results = %v count = %v", results, out["count"])
	}
	first, _ := results[0].(map[string]interface{})
	if first["id"] != "seg-1" {
		t.Fatalf("results[0] = %v", first)
	}

	// An API failure surfaces soft.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errors":[{"message":"internal error","field":null}]}`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()
	out, err = segmentlist.Execute(nil, nil, listsInputs())
	if err != nil {
		t.Fatalf("Execute 500: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "internal error") {
		t.Fatalf("500 result = %v", out)
	}
}

func TestListsSegmentGet(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"seg-1","name":"Active customers","query_dsl":"SELECT c.contact_id FROM contact_data c","contacts_count":10,"contacts_sample":[{"id":"c1"}],"query_version":"2"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := segmentget.Execute(nil, nil, listsInputs(strConn("segment_id", "seg-1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if gotPath != "/v3/marketing/segments/2.0/seg-1" {
		t.Fatalf("path = %q", gotPath)
	}
	if out["id"] != "seg-1" {
		t.Fatalf("id = %v", out["id"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["query_dsl"] != "SELECT c.contact_id FROM contact_data c" {
		t.Fatalf("result = %v", result)
	}
	if !strings.Contains(out["tool_result"].(string), "Active customers") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}

	// A bad id surfaces SendGrid's 404 soft.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"errors":[{"message":"segment not found","field":null}]}`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()
	out, err = segmentget.Execute(nil, nil, listsInputs(strConn("segment_id", "nope")))
	if err != nil {
		t.Fatalf("Execute 404: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "segment not found") {
		t.Fatalf("404 result = %v", out)
	}
}
