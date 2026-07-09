// Tests for the shared SendGrid client, plus an end-to-end exercise of the
// mail_send exemplar. This is an EXTERNAL test package (sendgrid_test) so it
// can import sibling action packages (which import sendgrid) without a cycle;
// sibling test files in this directory share the tiny input constructors
// below — declare them here only.
package sendgrid_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
	mailsend "flomation.app/automate/executor/actions/marketing/sendgrid/mail_send"
)

func strConn(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}
func secretConn(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeSecret, Value: val}
}
func textConn(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: val}
}
func objConn(name, jsonStr string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: jsonStr}
}
func boolConn(name string, val bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: val}
}
func dtConn(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeDateTime, Value: val}
}

func TestFoundationDoSetsBearerAndHeaders(t *testing.T) {
	var gotAuth, gotAccept, gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"list-uuid","name":"My List","contact_count":0}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	a := sendgrid.Auth{APIKey: "SG.TOK-XYZ"}
	result, _, status, err := sendgrid.Do(a, http.MethodPost, "/v3/marketing/lists", nil, map[string]interface{}{"name": "My List"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "Bearer SG.TOK-XYZ" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept header = %q", gotAccept)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type header = %q", gotContentType)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(gotBody), &parsed); err != nil || parsed["name"] != "My List" {
		t.Fatalf("request body = %q", gotBody)
	}
	if status != 201 {
		t.Fatalf("status = %d", status)
	}
	obj, ok := result.(map[string]interface{})
	if !ok || obj["id"] != "list-uuid" {
		t.Fatalf("decoded result = %v", result)
	}
}

func TestFoundationRedactsAPIKeyOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errors":[{"message":"key SECRETKEY123 is not valid","field":null}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	a := sendgrid.Auth{APIKey: "SECRETKEY123"}
	_, _, _, err := sendgrid.Do(a, http.MethodGet, "/v3/templates", nil, nil)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if strings.Contains(err.Error(), "SECRETKEY123") {
		t.Fatalf("API key not redacted: %v", err)
	}
	if !strings.Contains(err.Error(), "REDACTED") {
		t.Fatalf("redaction marker missing: %v", err)
	}
}

func TestFoundationErrorEnvelopeDecode(t *testing.T) {
	body := `{"errors":[{"message":"does not contain a valid address","field":"personalizations.0.to"},{"message":"too large","error_id":"invalid-request","parameter":"page_size"}]}`
	err := sendgrid.CheckResponse(&sendgrid.APIResponse{StatusCode: 400, Body: []byte(body)})
	if err == nil {
		t.Fatal("expected error for 400")
	}
	msg := err.Error()
	if !strings.Contains(msg, "personalizations.0.to: does not contain a valid address") {
		t.Fatalf("field: message not surfaced, got %v", msg)
	}
	if !strings.Contains(msg, "page_size: too large") {
		t.Fatalf("marketing parameter variant not surfaced, got %v", msg)
	}
	if !strings.Contains(msg, "400") {
		t.Fatalf("HTTP status missing, got %v", msg)
	}

	// Fallback: an unrecognised body is surfaced raw (truncated).
	err = sendgrid.CheckResponse(&sendgrid.APIResponse{StatusCode: 500, Body: []byte("upstream exploded")})
	if err == nil || !strings.Contains(err.Error(), "upstream exploded") {
		t.Fatalf("raw fallback not surfaced, got %v", err)
	}
}

func TestFoundationRateLimitMessage(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-RateLimit-Reset", "1783500000")
	err := sendgrid.CheckResponse(&sendgrid.APIResponse{
		StatusCode: 429,
		Body:       []byte(`{"errors":[{"field":null,"message":"too many requests"}]}`),
		Headers:    headers,
	})
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "1783500000") {
		t.Fatalf("rate limit message = %v", err)
	}
	err = sendgrid.CheckResponse(&sendgrid.APIResponse{StatusCode: 429, Headers: http.Header{}})
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("headerless rate limit message = %v", err)
	}
}

func TestFoundationRegionHostMapping(t *testing.T) {
	inputs := []*core.Connection{
		secretConn("api_key", "SG.tok"),
		strConn("region", "eu"),
	}
	a, err := sendgrid.GetAuth(inputs)
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if a.BaseURL() != "https://api.eu.sendgrid.com" {
		t.Fatalf("eu base = %q", a.BaseURL())
	}
	// Empty region defaults to the global host.
	a, err = sendgrid.GetAuth(inputs[:1])
	if err != nil {
		t.Fatalf("GetAuth default: %v", err)
	}
	if a.Region != "" || a.BaseURL() != "https://api.sendgrid.com" {
		t.Fatalf("default region = %q base = %q", a.Region, a.BaseURL())
	}
	// Anything else is rejected.
	if _, err = sendgrid.GetAuth([]*core.Connection{secretConn("api_key", "SG.tok"), strConn("region", "us")}); err == nil {
		t.Fatal("expected error for unknown region")
	}
}

func TestFoundationListMarketingFollowsNext(t *testing.T) {
	calls := 0
	var secondPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("page_token") == "" {
			// First page → _metadata.next carries SendGrid's OWN host; the
			// client must extract the query and re-issue against our host.
			_, _ = w.Write([]byte(`{"result":[{"id":"1"},{"id":"2"}],"_metadata":{"self":"x","next":"https://api.sendgrid.com/v3/marketing/lists?page_size=1000&page_token=tok2","count":3}}`))
		} else {
			secondPath = r.URL.Path
			if got := r.URL.Query().Get("page_token"); got != "tok2" {
				t.Errorf("page_token = %q", got)
			}
			if got := r.URL.Query().Get("page_size"); got != "1000" {
				t.Errorf("page_size = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":[{"id":"3"}],"_metadata":{"self":"x","count":3}}`))
		}
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	items, err := sendgrid.ListMarketing(sendgrid.Auth{APIKey: "t"}, "/v3/marketing/lists", nil, "result", 0, true)
	if err != nil {
		t.Fatalf("ListMarketing: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items across pages, got %d", len(items))
	}
	if calls != 2 {
		t.Fatalf("expected 2 page fetches, got %d", calls)
	}
	if secondPath != "/v3/marketing/lists" {
		t.Fatalf("next page re-issued against %q, want our fixed path", secondPath)
	}

	// returnAll=false → single page only, sized by limit.
	calls = 0
	single, err := sendgrid.ListMarketing(sendgrid.Auth{APIKey: "t"}, "/v3/marketing/lists", nil, "result", 2, false)
	if err != nil {
		t.Fatalf("ListMarketing single: %v", err)
	}
	if len(single) != 2 || calls != 1 {
		t.Fatalf("single page: got %d items in %d calls", len(single), calls)
	}
}

func TestFoundationListOffsetStopsOnShortPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.URL.Query().Get("limit"); got != "500" {
			t.Errorf("limit = %q", got)
		}
		switch r.URL.Query().Get("offset") {
		case "0":
			// A full page → there may be more.
			page := make([]map[string]interface{}, 500)
			for i := range page {
				page[i] = map[string]interface{}{"email": "a@x.com"}
			}
			_ = json.NewEncoder(w).Encode(page)
		case "500":
			// A short page → the collection is drained.
			_, _ = w.Write([]byte(`[{"email":"y@x.com"},{"email":"z@x.com"}]`))
		default:
			t.Errorf("unexpected offset %q", r.URL.Query().Get("offset"))
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	items, err := sendgrid.ListOffset(sendgrid.Auth{APIKey: "t"}, "/v3/suppression/bounces", nil, 0, true)
	if err != nil {
		t.Fatalf("ListOffset: %v", err)
	}
	if len(items) != 502 {
		t.Fatalf("expected 502 items across pages, got %d", len(items))
	}
	if calls != 2 {
		t.Fatalf("expected 2 page fetches, got %d", calls)
	}

	// returnAll=false → a single page sized by limit.
	calls = 0
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.URL.Query().Get("limit"); got != "7" {
			t.Errorf("single-page limit = %q", got)
		}
		_, _ = w.Write([]byte(`[{"email":"a@x.com"}]`))
	}))
	defer srv2.Close()
	defer sendgrid.SetBaseForTest(srv2.URL)()
	single, err := sendgrid.ListOffset(sendgrid.Auth{APIKey: "t"}, "/v3/suppression/bounces", nil, 7, false)
	if err != nil {
		t.Fatalf("ListOffset single: %v", err)
	}
	if len(single) != 1 || calls != 1 {
		t.Fatalf("single page: got %d items in %d calls", len(single), calls)
	}
}

func TestFoundationTopLevelArrayResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":100,"name":"Weekly digest"},{"id":101,"name":"Product news"}]`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	result, _, _, err := sendgrid.Do(sendgrid.Auth{APIKey: "t"}, http.MethodGet, "/v3/asm/groups", nil, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	arr, ok := result.([]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("top-level array not decoded: %v", result)
	}
}

func TestFoundationEmpty202WithHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Message-Id", "msg-abc123")
		w.WriteHeader(202)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	result, headers, status, err := sendgrid.Do(sendgrid.Auth{APIKey: "t"}, http.MethodPost, "/v3/mail/send", nil, map[string]interface{}{"from": map[string]interface{}{"email": "a@x.com"}})
	if err != nil {
		t.Fatalf("Do on 202 empty body: %v", err)
	}
	obj, ok := result.(map[string]interface{})
	if !ok || len(obj) != 0 {
		t.Fatalf("expected empty object, got %v", result)
	}
	if status != 202 {
		t.Fatalf("status = %d", status)
	}
	if headers.Get("X-Message-Id") != "msg-abc123" {
		t.Fatalf("X-Message-Id header = %q", headers.Get("X-Message-Id"))
	}
}

func TestFoundationClampLimit(t *testing.T) {
	if got := sendgrid.ClampLimit(0, false, sendgrid.DefaultPageLimit, sendgrid.MaxPageLimit); got != sendgrid.DefaultPageLimit {
		t.Fatalf("unset limit = %d", got)
	}
	if got := sendgrid.ClampLimit(5000, true, sendgrid.DefaultPageLimit, sendgrid.MaxPageLimit); got != sendgrid.MaxPageLimit {
		t.Fatalf("oversized limit = %d", got)
	}
	if got := sendgrid.ClampLimit(42, true, sendgrid.DefaultPageLimit, sendgrid.MaxPageLimit); got != 42 {
		t.Fatalf("in-range limit = %d", got)
	}
	if got := sendgrid.ClampLimit(700, true, sendgrid.DefaultPageLimit, sendgrid.MaxSuppressionPageLimit); got != sendgrid.MaxSuppressionPageLimit {
		t.Fatalf("suppression cap = %d", got)
	}
}

// mailSendInputs assembles the minimum viable mail_send inputs plus extras.
func mailSendInputs(extra ...*core.Connection) []*core.Connection {
	base := []*core.Connection{
		secretConn("api_key", "SG.testkey"),
		strConn("from_email", "sender@acme.com"),
		strConn("to", "a@x.com, b@y.com"),
	}
	return append(base, extra...)
}

func TestMailSendPlain(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v3/mail/send" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("X-Message-Id", "msg-abc123")
		w.WriteHeader(202)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := mailsend.Execute(nil, nil, mailSendInputs(
		strConn("from_name", "Acme Sales"),
		strConn("cc", "c@z.com"),
		strConn("subject", "Hello"),
		strConn("content_type", "text/plain"),
		textConn("content", "Hi there"),
		strConn("categories", "one, two"),
		strConn("asm_group_id", "42"),
		objConn("custom_args", `{"order_id":"12345"}`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if out["id"] != "msg-abc123" {
		t.Fatalf("id = %v", out["id"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["message_id"] != "msg-abc123" || result["status"] != "accepted" {
		t.Fatalf("result = %v", result)
	}

	from, _ := gotBody["from"].(map[string]interface{})
	if from["email"] != "sender@acme.com" || from["name"] != "Acme Sales" {
		t.Fatalf("from = %v", from)
	}
	personalizations, _ := gotBody["personalizations"].([]interface{})
	if len(personalizations) != 1 {
		t.Fatalf("personalizations = %v", gotBody["personalizations"])
	}
	p, _ := personalizations[0].(map[string]interface{})
	to, _ := p["to"].([]interface{})
	if len(to) != 2 || to[0].(map[string]interface{})["email"] != "a@x.com" || to[1].(map[string]interface{})["email"] != "b@y.com" {
		t.Fatalf("to = %v", p["to"])
	}
	cc, _ := p["cc"].([]interface{})
	if len(cc) != 1 || cc[0].(map[string]interface{})["email"] != "c@z.com" {
		t.Fatalf("cc = %v", p["cc"])
	}
	customArgs, _ := p["custom_args"].(map[string]interface{})
	if customArgs["order_id"] != "12345" {
		t.Fatalf("custom_args = %v", p["custom_args"])
	}
	if gotBody["subject"] != "Hello" {
		t.Fatalf("subject = %v", gotBody["subject"])
	}
	content, _ := gotBody["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("content = %v", gotBody["content"])
	}
	part, _ := content[0].(map[string]interface{})
	if part["type"] != "text/plain" || part["value"] != "Hi there" {
		t.Fatalf("content part = %v", part)
	}
	categories, _ := gotBody["categories"].([]interface{})
	if len(categories) != 2 || categories[0] != "one" || categories[1] != "two" {
		t.Fatalf("categories = %v", gotBody["categories"])
	}
	asm, _ := gotBody["asm"].(map[string]interface{})
	if asm["group_id"] != float64(42) {
		t.Fatalf("asm = %v", gotBody["asm"])
	}
	if _, ok := gotBody["template_id"]; ok {
		t.Fatalf("template_id sent on a direct-content send: %v", gotBody["template_id"])
	}
}

func TestMailSendContentOrdering(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("X-Message-Id", "msg-1")
		w.WriteHeader(202)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	// additional_fields supplies a content array in the WRONG MIME order —
	// SendGrid 400s unless text/plain precedes text/html, so it is re-ordered.
	out, err := mailsend.Execute(nil, nil, mailSendInputs(
		strConn("subject", "Hello"),
		textConn("content", "ignored"),
		objConn("additional_fields", `{"content":[{"type":"text/html","value":"<b>Hi</b>"},{"type":"text/plain","value":"Hi"}]}`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	content, _ := gotBody["content"].([]interface{})
	if len(content) != 2 {
		t.Fatalf("content = %v", gotBody["content"])
	}
	first, _ := content[0].(map[string]interface{})
	second, _ := content[1].(map[string]interface{})
	if first["type"] != "text/plain" || second["type"] != "text/html" {
		t.Fatalf("content order = %v, %v", first["type"], second["type"])
	}
}

func TestMailSendTemplate(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("X-Message-Id", "msg-2")
		w.WriteHeader(202)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := mailsend.Execute(nil, nil, mailSendInputs(
		boolConn("use_template", true),
		strConn("template_id", "d-abc123"),
		objConn("dynamic_template_data", `{"first_name":"Jane"}`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	if gotBody["template_id"] != "d-abc123" {
		t.Fatalf("template_id = %v", gotBody["template_id"])
	}
	if _, ok := gotBody["subject"]; ok {
		t.Fatalf("subject sent on a template send: %v", gotBody["subject"])
	}
	if _, ok := gotBody["content"]; ok {
		t.Fatalf("content sent on a template send: %v", gotBody["content"])
	}
	personalizations, _ := gotBody["personalizations"].([]interface{})
	p, _ := personalizations[0].(map[string]interface{})
	data, _ := p["dynamic_template_data"].(map[string]interface{})
	if data["first_name"] != "Jane" {
		t.Fatalf("dynamic_template_data = %v", p["dynamic_template_data"])
	}
}

func TestMailSendSendAtEpoch(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("X-Message-Id", "msg-3")
		w.WriteHeader(202)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := mailsend.Execute(nil, nil, mailSendInputs(
		strConn("subject", "Hello"),
		textConn("content", "Hi"),
		dtConn("send_at", "2026-07-10T09:00:00Z"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error %v)", out["success"], out["error"])
	}
	want, _ := time.Parse(time.RFC3339, "2026-07-10T09:00:00Z")
	personalizations, _ := gotBody["personalizations"].([]interface{})
	p, _ := personalizations[0].(map[string]interface{})
	if p["send_at"] != float64(want.Unix()) {
		t.Fatalf("send_at = %v, want %d", p["send_at"], want.Unix())
	}

	// An unparseable date is a soft failure, not a dropped field.
	out, err = mailsend.Execute(nil, nil, mailSendInputs(
		strConn("subject", "Hello"),
		textConn("content", "Hi"),
		dtConn("send_at", "next tuesday"),
	))
	if err != nil {
		t.Fatalf("Execute bad send_at: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "send_at") {
		t.Fatalf("bad send_at result = %v", out)
	}
}

func TestMailSendMissingSubjectValidation(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(202)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	out, err := mailsend.Execute(nil, nil, mailSendInputs(
		textConn("content", "Hi"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("missing subject accepted: %v", out)
	}
	if !strings.Contains(out["error"].(string), "Subject") {
		t.Fatalf("error = %v", out["error"])
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}
