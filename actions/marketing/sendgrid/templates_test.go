// Tests for the templates/senders cluster: template CRUD, template versions,
// and verified senders. Shares the input constructors declared in
// common_test.go.
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
	senderlist "flomation.app/automate/executor/actions/marketing/sendgrid/sender_list"
	tmplcreate "flomation.app/automate/executor/actions/marketing/sendgrid/template_create"
	tmpldelete "flomation.app/automate/executor/actions/marketing/sendgrid/template_delete"
	tmplget "flomation.app/automate/executor/actions/marketing/sendgrid/template_get"
	tmpllist "flomation.app/automate/executor/actions/marketing/sendgrid/template_list"
	tmplupdate "flomation.app/automate/executor/actions/marketing/sendgrid/template_update"
	tmplversionactivate "flomation.app/automate/executor/actions/marketing/sendgrid/template_version_activate"
	tmplversioncreate "flomation.app/automate/executor/actions/marketing/sendgrid/template_version_create"
)

// templatesInputs assembles the auth input plus the given extras.
func templatesInputs(extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{secretConn("api_key", "SG.testkey")}, extra...)
}

func TestTemplatesCreate(t *testing.T) {
	calls := 0
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/v3/templates" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"d-tmpl1","name":"Order Confirmation","generation":"dynamic","updated_at":"2026-07-09 10:00:00","versions":[]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := tmplcreate.Execute(nil, nil, templatesInputs(strConn("name", "Order Confirmation")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "d-tmpl1" {
		t.Fatalf("id = %v", out["id"])
	}
	if gotBody["name"] != "Order Confirmation" {
		t.Fatalf("name = %v", gotBody["name"])
	}
	if gotBody["generation"] != "dynamic" {
		t.Fatalf("generation = %v — new templates must default to dynamic", gotBody["generation"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["name"] != "Order Confirmation" {
		t.Fatalf("result = %v", result)
	}

	// Missing name is a soft failure before any request.
	calls = 0
	out, err = tmplcreate.Execute(nil, nil, templatesInputs())
	if err != nil {
		t.Fatalf("Execute missing name: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "name") {
		t.Fatalf("missing name result = %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestTemplatesGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/templates/d-tmpl1" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"d-tmpl1","name":"Order Confirmation","generation":"dynamic","versions":[{"id":"v-1","active":1,"name":"first"}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := tmplget.Execute(nil, nil, templatesInputs(strConn("template_id", "d-tmpl1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "d-tmpl1" {
		t.Fatalf("id = %v", out["id"])
	}
	result, _ := out["result"].(map[string]interface{})
	versions, _ := result["versions"].([]interface{})
	if len(versions) != 1 {
		t.Fatalf("versions = %v", result["versions"])
	}

	// An unknown template surfaces the API error softly.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"errors":[{"message":"Template not found","field":null}]}`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()
	out, err = tmplget.Execute(nil, nil, templatesInputs(strConn("template_id", "d-missing")))
	if err != nil {
		t.Fatalf("Execute 404: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "Template not found") {
		t.Fatalf("404 result = %v", out)
	}
}

func TestTemplatesListDefaults(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v3/templates" {
			t.Errorf("path = %s", r.URL.Path)
		}
		// generations defaults to legacy at SendGrid, so the client must ask
		// for dynamic explicitly; page_size is REQUIRED by the endpoint.
		if got := r.URL.Query().Get("generations"); got != "dynamic" {
			t.Errorf("generations = %q, want dynamic default", got)
		}
		if got := r.URL.Query().Get("page_size"); got != "100" {
			t.Errorf("page_size = %q, want the default page size", got)
		}
		_, _ = w.Write([]byte(`{"result":[{"id":"d-1","name":"One"},{"id":"d-2","name":"Two"}],"_metadata":{"self":"x","count":2}}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := tmpllist.Execute(nil, nil, templatesInputs())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 2 || out["count"] != 2 {
		t.Fatalf("results = %v count = %v", out["results"], out["count"])
	}
	if calls != 1 {
		t.Fatalf("expected a single page fetch, got %d", calls)
	}

	// An API failure is a soft failure.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errors":[{"message":"page_size is required","field":"page_size"}]}`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()
	out, err = tmpllist.Execute(nil, nil, templatesInputs())
	if err != nil {
		t.Fatalf("Execute error path: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "page_size") {
		t.Fatalf("error result = %v", out)
	}
}

func TestTemplatesListReturnAllFollowsNext(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Templates cap page_size at 200 — return-all must page at the cap.
		if got := r.URL.Query().Get("page_size"); got != "200" {
			t.Errorf("page_size = %q, want the 200 template cap", got)
		}
		if got := r.URL.Query().Get("generations"); got != "dynamic" {
			t.Errorf("generations = %q", got)
		}
		if r.URL.Query().Get("page_token") == "" {
			_, _ = w.Write([]byte(`{"result":[{"id":"d-1"},{"id":"d-2"}],"_metadata":{"self":"x","next":"https://api.sendgrid.com/v3/templates?generations=dynamic&page_size=200&page_token=tok2","count":3}}`))
		} else {
			if got := r.URL.Query().Get("page_token"); got != "tok2" {
				t.Errorf("page_token = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":[{"id":"d-3"}],"_metadata":{"self":"x","count":3}}`))
		}
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := tmpllist.Execute(nil, nil, templatesInputs(boolConn("return_all", true)))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 3 || out["count"] != 3 {
		t.Fatalf("results = %v count = %v", out["results"], out["count"])
	}
	if calls != 2 {
		t.Fatalf("expected 2 page fetches, got %d", calls)
	}
}

func TestTemplatesListEnvelopeFallback(t *testing.T) {
	// The documented envelope key is "result", but the legacy shape has been
	// seen keyed "templates" — the unwrap must tolerate both.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("generations"); got != "legacy,dynamic" {
			t.Errorf("generations = %q, want the explicit selection passed through", got)
		}
		if r.URL.Query().Get("page_size") == "" {
			t.Error("page_size missing — the endpoint requires it")
		}
		_, _ = w.Write([]byte(`{"templates":[{"id":"legacy-1","name":"Old"},{"id":"d-1","name":"New"}],"_metadata":{"self":"x","count":2}}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := tmpllist.Execute(nil, nil, templatesInputs(strConn("generations", "legacy,dynamic")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 2 {
		t.Fatalf("templates-keyed envelope not unwrapped: %v", out["results"])
	}
}

func TestTemplatesUpdate(t *testing.T) {
	calls := 0
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPatch || r.URL.Path != "/v3/templates/d-tmpl1" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"id":"d-tmpl1","name":"Renamed","generation":"dynamic","versions":[]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := tmplupdate.Execute(nil, nil, templatesInputs(
		strConn("template_id", "d-tmpl1"),
		strConn("name", "Renamed"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "d-tmpl1" {
		t.Fatalf("id = %v", out["id"])
	}
	if gotBody["name"] != "Renamed" {
		t.Fatalf("body = %v", gotBody)
	}

	// Missing name is a soft failure before any request.
	calls = 0
	out, err = tmplupdate.Execute(nil, nil, templatesInputs(strConn("template_id", "d-tmpl1")))
	if err != nil {
		t.Fatalf("Execute missing name: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "name") {
		t.Fatalf("missing name result = %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestTemplatesDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v3/templates/d-tmpl1" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := tmpldelete.Execute(nil, nil, templatesInputs(strConn("template_id", "d-tmpl1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "d-tmpl1" {
		t.Fatalf("id = %v", out["id"])
	}

	// An unknown template surfaces the API error softly.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"errors":[{"message":"Template not found","field":null}]}`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()
	out, err = tmpldelete.Execute(nil, nil, templatesInputs(strConn("template_id", "d-missing")))
	if err != nil {
		t.Fatalf("Execute 404: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "404") {
		t.Fatalf("404 result = %v", out)
	}
}

func TestTemplatesVersionCreate(t *testing.T) {
	calls := 0
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/v3/templates/d-tmpl1/versions" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = nil
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"v-new","template_id":"d-tmpl1","active":1,"name":"July redesign","subject":"Your order"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := tmplversioncreate.Execute(nil, nil, templatesInputs(
		strConn("template_id", "d-tmpl1"),
		strConn("name", "July redesign"),
		strConn("subject", "Your order"),
		textConn("html_content", "<h1>Thanks {{first_name}}</h1>"),
		boolConn("generate_plain_content", true),
		boolConn("active", true),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "v-new" {
		t.Fatalf("id = %v", out["id"])
	}
	if gotBody["name"] != "July redesign" || gotBody["subject"] != "Your order" {
		t.Fatalf("body = %v", gotBody)
	}
	if gotBody["html_content"] != "<h1>Thanks {{first_name}}</h1>" {
		t.Fatalf("html_content = %v", gotBody["html_content"])
	}
	if gotBody["generate_plain_content"] != true {
		t.Fatalf("generate_plain_content = %v", gotBody["generate_plain_content"])
	}
	// The API takes active as an INTEGER flag, not a boolean.
	if gotBody["active"] != float64(1) {
		t.Fatalf("active = %v (%T), want the integer 1", gotBody["active"], gotBody["active"])
	}

	// Untouched checkboxes stay out of the body entirely.
	out, err = tmplversioncreate.Execute(nil, nil, templatesInputs(
		strConn("template_id", "d-tmpl1"),
		strConn("name", "minimal"),
	))
	if err != nil {
		t.Fatalf("Execute minimal: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("minimal success = %v (error %v)", out["success"], out["error"])
	}
	if _, ok := gotBody["active"]; ok {
		t.Fatalf("active sent when unset: %v", gotBody["active"])
	}
	if _, ok := gotBody["generate_plain_content"]; ok {
		t.Fatalf("generate_plain_content sent when unset: %v", gotBody["generate_plain_content"])
	}

	// Missing name is a soft failure before any request.
	calls = 0
	out, err = tmplversioncreate.Execute(nil, nil, templatesInputs(strConn("template_id", "d-tmpl1")))
	if err != nil {
		t.Fatalf("Execute missing name: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "name") {
		t.Fatalf("missing name result = %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestTemplatesVersionActivate(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Activation is a POST (not a PATCH) with an EMPTY body.
		if r.Method != http.MethodPost || r.URL.Path != "/v3/templates/d-tmpl1/versions/v-2/activate" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if len(b) != 0 {
			t.Errorf("activate body = %q, want empty", string(b))
		}
		_, _ = w.Write([]byte(`{"id":"v-2","template_id":"d-tmpl1","active":1,"name":"July redesign"}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := tmplversionactivate.Execute(nil, nil, templatesInputs(
		strConn("template_id", "d-tmpl1"),
		strConn("version_id", "v-2"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "v-2" {
		t.Fatalf("id = %v", out["id"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["active"] != float64(1) {
		t.Fatalf("result = %v", result)
	}

	// Missing version_id is a soft failure before any request.
	calls = 0
	out, err = tmplversionactivate.Execute(nil, nil, templatesInputs(strConn("template_id", "d-tmpl1")))
	if err != nil {
		t.Fatalf("Execute missing version: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "version_id") {
		t.Fatalf("missing version result = %v", out)
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestTemplatesSenderList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v3/verified_senders" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":1,"from_email":"a@acme.com","verified":true},{"id":2,"from_email":"b@acme.com","verified":false}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := senderlist.Execute(nil, nil, templatesInputs())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	results, _ := out["results"].([]interface{})
	if len(results) != 2 || out["count"] != 2 {
		t.Fatalf("results = %v count = %v", out["results"], out["count"])
	}
	first, _ := results[0].(map[string]interface{})
	if first["from_email"] != "a@acme.com" {
		t.Fatalf("first sender = %v", first)
	}

	// The limit clamps client-side.
	out, err = senderlist.Execute(nil, nil, templatesInputs(strConn("limit", "1")))
	if err != nil {
		t.Fatalf("Execute with limit: %v", err)
	}
	results, _ = out["results"].([]interface{})
	if len(results) != 1 || out["count"] != 1 {
		t.Fatalf("clamped results = %v count = %v", out["results"], out["count"])
	}

	// A bad key surfaces the API error softly.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errors":[{"message":"authorization required","field":null}]}`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()
	out, err = senderlist.Execute(nil, nil, templatesInputs())
	if err != nil {
		t.Fatalf("Execute 401: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "authorization required") {
		t.Fatalf("401 result = %v", out)
	}
}
