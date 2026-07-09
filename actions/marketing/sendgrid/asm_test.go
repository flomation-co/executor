// Tests for the Global-unsubscribe / ASM-group / stats action cluster.
// Shared input constructors (strConn, secretConn, ...) live in common_test.go.
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
	groupcreate "flomation.app/automate/executor/actions/marketing/sendgrid/asm_group_create"
	groupdelete "flomation.app/automate/executor/actions/marketing/sendgrid/asm_group_delete"
	groupget "flomation.app/automate/executor/actions/marketing/sendgrid/asm_group_get"
	grouplist "flomation.app/automate/executor/actions/marketing/sendgrid/asm_group_list"
	groupupdate "flomation.app/automate/executor/actions/marketing/sendgrid/asm_group_update"
	suppadd "flomation.app/automate/executor/actions/marketing/sendgrid/asm_suppression_add"
	suppdelete "flomation.app/automate/executor/actions/marketing/sendgrid/asm_suppression_delete"
	supplist "flomation.app/automate/executor/actions/marketing/sendgrid/asm_suppression_list"
	guadd "flomation.app/automate/executor/actions/marketing/sendgrid/global_unsubscribe_add"
	gucheck "flomation.app/automate/executor/actions/marketing/sendgrid/global_unsubscribe_check"
	gudelete "flomation.app/automate/executor/actions/marketing/sendgrid/global_unsubscribe_delete"
	statsget "flomation.app/automate/executor/actions/marketing/sendgrid/stats_get"
)

// asmInputs assembles the auth input plus the action's own inputs.
func asmInputs(extra ...*core.Connection) []*core.Connection {
	base := []*core.Connection{secretConn("api_key", "SG.testkey")}
	return append(base, extra...)
}

func TestAsmGlobalUnsubscribeAdd(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v3/asm/suppressions/global" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"recipient_emails":["a@x.com","b@y.com"]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := guadd.Execute(nil, nil, asmInputs(strConn("emails", "a@x.com, b@y.com")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	recipients, _ := gotBody["recipient_emails"].([]interface{})
	if len(recipients) != 2 || recipients[0] != "a@x.com" || recipients[1] != "b@y.com" {
		t.Fatalf("recipient_emails = %v", gotBody["recipient_emails"])
	}
	result, _ := out["result"].(map[string]interface{})
	echoed, _ := result["recipient_emails"].([]interface{})
	if len(echoed) != 2 {
		t.Fatalf("result = %v", out["result"])
	}
	if !strings.Contains(out["tool_result"].(string), "2 email(s)") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}
}

func TestAsmGlobalUnsubscribeAddMissingEmails(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(201)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := guadd.Execute(nil, nil, asmInputs())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "emails") {
		t.Fatalf("missing emails accepted: %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestAsmGlobalUnsubscribeCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		switch r.URL.Path {
		case "/v3/asm/suppressions/global/gone@x.com":
			_, _ = w.Write([]byte(`{"recipient_email":"gone@x.com"}`))
		case "/v3/asm/suppressions/global/fine@x.com":
			// Not suppressed: the API answers 200 with an EMPTY object.
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("path = %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := gucheck.Execute(nil, nil, asmInputs(strConn("email", "gone@x.com")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "gone@x.com" {
		t.Fatalf("suppressed check = %v", out)
	}
	result, _ := out["result"].(map[string]interface{})
	if result["suppressed"] != true || result["email"] != "gone@x.com" {
		t.Fatalf("suppressed result = %v", result)
	}
	if !strings.Contains(out["tool_result"].(string), "is on the global unsubscribe list") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}

	// A 200 {} means NOT suppressed — a normal success, never an error.
	out, err = gucheck.Execute(nil, nil, asmInputs(strConn("email", "fine@x.com")))
	if err != nil {
		t.Fatalf("Execute not-suppressed: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("empty-object check treated as failure: %v", out)
	}
	result, _ = out["result"].(map[string]interface{})
	if result["suppressed"] != false || result["email"] != "fine@x.com" {
		t.Fatalf("not-suppressed result = %v", result)
	}
	if !strings.Contains(out["tool_result"].(string), "is not on the global unsubscribe list") {
		t.Fatalf("tool_result = %v", out["tool_result"])
	}
}

func TestAsmGlobalUnsubscribeCheckAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errors":[{"message":"internal error","field":null}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := gucheck.Execute(nil, nil, asmInputs(strConn("email", "a@x.com")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "internal error") {
		t.Fatalf("500 not surfaced: %v", out)
	}
}

func TestAsmGlobalUnsubscribeDelete(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := gudelete.Execute(nil, nil, asmInputs(strConn("email", "a@x.com")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "a@x.com" {
		t.Fatalf("delete result = %v", out)
	}
	if gotMethod != http.MethodDelete || gotPath != "/v3/asm/suppressions/global/a@x.com" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}

	// Missing email is a soft validation failure.
	out, err = gudelete.Execute(nil, nil, asmInputs())
	if err != nil {
		t.Fatalf("Execute missing email: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "email") {
		t.Fatalf("missing email accepted: %v", out)
	}
}

func TestAsmGroupCreate(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v3/asm/groups" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":100,"name":"Weekly digest","description":"Our weekly roundup","is_default":false}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := groupcreate.Execute(nil, nil, asmInputs(
		strConn("name", "Weekly digest"),
		strConn("description", "Our weekly roundup"),
		boolConn("is_default", false),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	// The numeric group id is stringified.
	if out["id"] != "100" {
		t.Fatalf("id = %v", out["id"])
	}
	if gotBody["name"] != "Weekly digest" || gotBody["description"] != "Our weekly roundup" || gotBody["is_default"] != false {
		t.Fatalf("body = %v", gotBody)
	}
}

func TestAsmGroupCreateAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errors":[{"message":"group name already exists","field":"name"}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := groupcreate.Execute(nil, nil, asmInputs(strConn("name", "Weekly digest")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "name: group name already exists") {
		t.Fatalf("400 not surfaced: %v", out)
	}
}

func TestAsmGroupGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/asm/groups/100" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":100,"name":"Weekly digest","description":"Our weekly roundup","is_default":false,"unsubscribes":7}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := groupget.Execute(nil, nil, asmInputs(strConn("group_id", "100")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "100" {
		t.Fatalf("get result = %v", out)
	}
	result, _ := out["result"].(map[string]interface{})
	if result["name"] != "Weekly digest" || result["unsubscribes"] != float64(7) {
		t.Fatalf("result = %v", result)
	}

	// A non-numeric group id is rejected before any request.
	out, err = groupget.Execute(nil, nil, asmInputs(strConn("group_id", "weekly")))
	if err != nil {
		t.Fatalf("Execute bad group_id: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "whole number") {
		t.Fatalf("bad group_id accepted: %v", out)
	}
}

func TestAsmGroupList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/asm/groups" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		// A bare top-level array, no envelope.
		_, _ = w.Write([]byte(`[{"id":100,"name":"Weekly digest"},{"id":101,"name":"Product news"}]`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := grouplist.Execute(nil, nil, asmInputs())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["count"] != 2 {
		t.Fatalf("list result = %v", out)
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 2 || results[1].(map[string]interface{})["name"] != "Product news" {
		t.Fatalf("results = %v", out["results"])
	}
}

func TestAsmGroupListAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errors":[{"message":"authorization required","field":null}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := grouplist.Execute(nil, nil, asmInputs())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "authorization required") {
		t.Fatalf("401 not surfaced: %v", out)
	}
}

func TestAsmGroupUpdateReturns201(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		// Quirk: this PATCH answers 201, not 200.
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":100,"name":"Fortnightly digest","description":"Our roundup","is_default":true,"unsubscribes":7}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := groupupdate.Execute(nil, nil, asmInputs(
		strConn("group_id", "100"),
		strConn("name", "Fortnightly digest"),
		boolConn("is_default", true),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("201 PATCH treated as failure: %v", out)
	}
	if gotMethod != http.MethodPatch || gotPath != "/v3/asm/groups/100" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if gotBody["name"] != "Fortnightly digest" || gotBody["is_default"] != true {
		t.Fatalf("body = %v", gotBody)
	}
	if _, ok := gotBody["description"]; ok {
		t.Fatalf("unset description sent: %v", gotBody)
	}
	result, _ := out["result"].(map[string]interface{})
	if result["name"] != "Fortnightly digest" {
		t.Fatalf("result = %v", result)
	}
}

func TestAsmGroupUpdateNoFields(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(201)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := groupupdate.Execute(nil, nil, asmInputs(strConn("group_id", "100")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "provide") {
		t.Fatalf("empty update accepted: %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestAsmGroupDelete(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := groupdelete.Execute(nil, nil, asmInputs(strConn("group_id", "100")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "100" {
		t.Fatalf("delete result = %v", out)
	}
	if gotMethod != http.MethodDelete || gotPath != "/v3/asm/groups/100" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}

	// Missing group id is a soft validation failure.
	out, err = groupdelete.Execute(nil, nil, asmInputs())
	if err != nil {
		t.Fatalf("Execute missing group_id: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "group_id") {
		t.Fatalf("missing group_id accepted: %v", out)
	}
}

func TestAsmSuppressionAdd(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v3/asm/groups/42/suppressions" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"recipient_emails":["a@x.com","b@y.com"]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := suppadd.Execute(nil, nil, asmInputs(
		strConn("group_id", "42"),
		strConn("emails", "a@x.com, b@y.com"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "42" {
		t.Fatalf("add result = %v", out)
	}
	recipients, _ := gotBody["recipient_emails"].([]interface{})
	if len(recipients) != 2 || recipients[0] != "a@x.com" {
		t.Fatalf("recipient_emails = %v", gotBody["recipient_emails"])
	}

	// A non-numeric group id is rejected before any request.
	out, err = suppadd.Execute(nil, nil, asmInputs(
		strConn("group_id", "weekly"),
		strConn("emails", "a@x.com"),
	))
	if err != nil {
		t.Fatalf("Execute bad group_id: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "whole number") {
		t.Fatalf("bad group_id accepted: %v", out)
	}
}

func TestAsmSuppressionListWrapsStrings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/asm/groups/42/suppressions" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		// A bare array of plain email STRINGS.
		_, _ = w.Write([]byte(`["a@x.com","b@y.com"]`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := supplist.Execute(nil, nil, asmInputs(strConn("group_id", "42")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["count"] != 2 {
		t.Fatalf("list result = %v", out)
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 2 {
		t.Fatalf("results = %v", out["results"])
	}
	first, _ := results[0].(map[string]interface{})
	second, _ := results[1].(map[string]interface{})
	if first["email"] != "a@x.com" || second["email"] != "b@y.com" {
		t.Fatalf("strings not wrapped as {email}: %v", out["results"])
	}
}

func TestAsmSuppressionListAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"errors":[{"message":"group not found","field":null}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := supplist.Execute(nil, nil, asmInputs(strConn("group_id", "42")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "group not found") {
		t.Fatalf("404 not surfaced: %v", out)
	}
}

func TestAsmSuppressionDelete(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := suppdelete.Execute(nil, nil, asmInputs(
		strConn("group_id", "42"),
		strConn("email", "a@x.com"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "a@x.com" {
		t.Fatalf("delete result = %v", out)
	}
	if gotMethod != http.MethodDelete || gotPath != "/v3/asm/groups/42/suppressions/a@x.com" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}

	// Missing email is a soft validation failure.
	out, err = suppdelete.Execute(nil, nil, asmInputs(strConn("group_id", "42")))
	if err != nil {
		t.Fatalf("Execute missing email: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "email") {
		t.Fatalf("missing email accepted: %v", out)
	}
}

func TestAsmStatsGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/stats" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("start_date") != "2026-07-01" || q.Get("end_date") != "2026-07-08" || q.Get("aggregated_by") != "day" {
			t.Errorf("query = %v", q)
		}
		// A bare top-level array of {date, stats[]} buckets.
		_, _ = w.Write([]byte(`[{"date":"2026-07-01","stats":[{"metrics":{"delivered":5,"opens":3}}]},{"date":"2026-07-02","stats":[{"metrics":{"delivered":2,"opens":1}}]}]`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := statsget.Execute(nil, nil, asmInputs(
		strConn("start_date", "2026-07-01"),
		strConn("end_date", "2026-07-08"),
		strConn("aggregated_by", "day"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["count"] != 2 {
		t.Fatalf("stats result = %v", out)
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 2 || results[0].(map[string]interface{})["date"] != "2026-07-01" {
		t.Fatalf("results = %v", out["results"])
	}
}

func TestAsmStatsGetDateValidation(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	// A non-ISO date is rejected before any request.
	out, err := statsget.Execute(nil, nil, asmInputs(strConn("start_date", "01/07/2026")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "YYYY-MM-DD") {
		t.Fatalf("bad start_date accepted: %v", out)
	}

	// So is a date Go would parse leniently but SendGrid rejects.
	out, err = statsget.Execute(nil, nil, asmInputs(strConn("start_date", "2026-7-1")))
	if err != nil {
		t.Fatalf("Execute lenient date: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "YYYY-MM-DD") {
		t.Fatalf("lenient start_date accepted: %v", out)
	}

	// A bad end_date is rejected too.
	out, err = statsget.Execute(nil, nil, asmInputs(
		strConn("start_date", "2026-07-01"),
		strConn("end_date", "tomorrow"),
	))
	if err != nil {
		t.Fatalf("Execute bad end_date: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "end_date") {
		t.Fatalf("bad end_date accepted: %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}
