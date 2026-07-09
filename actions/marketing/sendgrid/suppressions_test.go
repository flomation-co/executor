// Tests for the legacy suppression cluster (bounces, blocks, spam reports,
// invalid emails). The four collections are structurally identical — top-level
// JSON arrays with limit/offset pagination, an array-shaped per-email get, and
// a three-mode delete — so every test runs across all four resources.
package sendgrid_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
	blockdelete "flomation.app/automate/executor/actions/marketing/sendgrid/block_delete"
	blockget "flomation.app/automate/executor/actions/marketing/sendgrid/block_get"
	blocklist "flomation.app/automate/executor/actions/marketing/sendgrid/block_list"
	bouncedelete "flomation.app/automate/executor/actions/marketing/sendgrid/bounce_delete"
	bounceget "flomation.app/automate/executor/actions/marketing/sendgrid/bounce_get"
	bouncelist "flomation.app/automate/executor/actions/marketing/sendgrid/bounce_list"
	invalidemaildelete "flomation.app/automate/executor/actions/marketing/sendgrid/invalid_email_delete"
	invalidemailget "flomation.app/automate/executor/actions/marketing/sendgrid/invalid_email_get"
	invalidemaillist "flomation.app/automate/executor/actions/marketing/sendgrid/invalid_email_list"
	spamreportdelete "flomation.app/automate/executor/actions/marketing/sendgrid/spam_report_delete"
	spamreportget "flomation.app/automate/executor/actions/marketing/sendgrid/spam_report_get"
	spamreportlist "flomation.app/automate/executor/actions/marketing/sendgrid/spam_report_list"
)

type suppressionExec func(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error)

var suppressionResources = []struct {
	name string
	path string
	list suppressionExec
	get  suppressionExec
	del  suppressionExec
}{
	{"bounce", "/v3/suppression/bounces", bouncelist.Execute, bounceget.Execute, bouncedelete.Execute},
	{"block", "/v3/suppression/blocks", blocklist.Execute, blockget.Execute, blockdelete.Execute},
	{"spam_report", "/v3/suppression/spam_reports", spamreportlist.Execute, spamreportget.Execute, spamreportdelete.Execute},
	{"invalid_email", "/v3/suppression/invalid_emails", invalidemaillist.Execute, invalidemailget.Execute, invalidemaildelete.Execute},
}

func TestSuppressionsListHappyPath(t *testing.T) {
	wantStart, _ := time.Parse(time.RFC3339, "2026-07-01T00:00:00Z")
	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			var gotQuery url.Values
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != tc.path {
					t.Errorf("request = %s %s, want GET %s", r.Method, r.URL.Path, tc.path)
				}
				gotQuery = r.URL.Query()
				_, _ = w.Write([]byte(`[{"created":1751328000,"email":"a@x.com","reason":"550 mailbox unavailable"},{"created":1751330000,"email":"b@y.com","reason":"550"}]`))
			}))
			defer srv.Close()
			defer sendgrid.SetBaseForTest(srv.URL)()

			out, err := tc.list(nil, nil, []*core.Connection{
				secretConn("api_key", "SG.testkey"),
				dtConn("start_time", "2026-07-01T00:00:00Z"),
				dtConn("end_time", "1783500000"),
				strConn("limit", "50"),
			})
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
			if first["email"] != "a@x.com" {
				t.Fatalf("first item = %v", results[0])
			}
			if got := gotQuery.Get("start_time"); got != strconv.FormatInt(wantStart.Unix(), 10) {
				t.Fatalf("start_time = %q, want %d", got, wantStart.Unix())
			}
			if got := gotQuery.Get("end_time"); got != "1783500000" {
				t.Fatalf("end_time = %q", got)
			}
			if got := gotQuery.Get("limit"); got != "50" {
				t.Fatalf("limit = %q", got)
			}
		})
	}
}

func TestSuppressionsListBadTimeFilter(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.list(nil, nil, []*core.Connection{
				secretConn("api_key", "SG.testkey"),
				dtConn("start_time", "next tuesday"),
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), "start_time") {
				t.Fatalf("bad start_time result = %v", out)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestSuppressionsListAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"errors":[{"message":"authorization required","field":null}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.list(nil, nil, []*core.Connection{secretConn("api_key", "SG.testkey")})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), "authorization required") {
				t.Fatalf("API error result = %v", out)
			}
		})
	}
}

func TestSuppressionsGetFound(t *testing.T) {
	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != tc.path+"/jane@example.com" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				_, _ = w.Write([]byte(`[{"created":1751328000,"email":"jane@example.com","reason":"550 mailbox unavailable"}]`))
			}))
			defer srv.Close()
			defer sendgrid.SetBaseForTest(srv.URL)()

			out, err := tc.get(nil, nil, []*core.Connection{
				secretConn("api_key", "SG.testkey"),
				strConn("email", "jane@example.com"),
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != true {
				t.Fatalf("success = %v (error %v)", out["success"], out["error"])
			}
			if out["id"] != "jane@example.com" {
				t.Fatalf("id = %v", out["id"])
			}
			result, _ := out["result"].(map[string]interface{})
			if result["email"] != "jane@example.com" {
				t.Fatalf("result = %v", out["result"])
			}
		})
	}
}

func TestSuppressionsGetEmptyArrayIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.get(nil, nil, []*core.Connection{
				secretConn("api_key", "SG.testkey"),
				strConn("email", "ghost@example.com"),
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), "found for ghost@example.com") {
				t.Fatalf("empty-array result = %v", out)
			}
		})
	}
}

func TestSuppressionsGetMissingEmail(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.get(nil, nil, []*core.Connection{secretConn("api_key", "SG.testkey")})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), "email is required") {
				t.Fatalf("missing email result = %v", out)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestSuppressionsDeleteSingleEmailPrecedence(t *testing.T) {
	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath, gotBody string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				w.WriteHeader(204)
			}))
			defer srv.Close()
			defer sendgrid.SetBaseForTest(srv.URL)()

			// A single Email wins over the bulk inputs.
			out, err := tc.del(nil, nil, []*core.Connection{
				secretConn("api_key", "SG.testkey"),
				strConn("email", "jane@example.com"),
				strConn("emails", "a@x.com, b@y.com"),
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != true {
				t.Fatalf("success = %v (error %v)", out["success"], out["error"])
			}
			if out["id"] != "jane@example.com" {
				t.Fatalf("id = %v", out["id"])
			}
			if gotMethod != http.MethodDelete || gotPath != tc.path+"/jane@example.com" {
				t.Fatalf("request = %s %s", gotMethod, gotPath)
			}
			if gotBody != "" {
				t.Fatalf("single-email delete sent a body: %q", gotBody)
			}
		})
	}
}

func TestSuppressionsDeleteBulkEmails(t *testing.T) {
	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			var gotBody map[string]interface{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &gotBody)
				w.WriteHeader(204)
			}))
			defer srv.Close()
			defer sendgrid.SetBaseForTest(srv.URL)()

			out, err := tc.del(nil, nil, []*core.Connection{
				secretConn("api_key", "SG.testkey"),
				strConn("emails", "a@x.com, b@y.com"),
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != true {
				t.Fatalf("success = %v (error %v)", out["success"], out["error"])
			}
			if gotMethod != http.MethodDelete || gotPath != tc.path {
				t.Fatalf("request = %s %s", gotMethod, gotPath)
			}
			emails, _ := gotBody["emails"].([]interface{})
			if len(emails) != 2 || emails[0] != "a@x.com" || emails[1] != "b@y.com" {
				t.Fatalf("body emails = %v", gotBody["emails"])
			}
			if _, ok := gotBody["delete_all"]; ok {
				t.Fatalf("delete_all sent alongside emails: %v", gotBody)
			}
		})
	}
}

func TestSuppressionsDeleteAll(t *testing.T) {
	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]interface{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != tc.path {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &gotBody)
				w.WriteHeader(204)
			}))
			defer srv.Close()
			defer sendgrid.SetBaseForTest(srv.URL)()

			out, err := tc.del(nil, nil, []*core.Connection{
				secretConn("api_key", "SG.testkey"),
				boolConn("delete_all", true),
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != true {
				t.Fatalf("success = %v (error %v)", out["success"], out["error"])
			}
			if gotBody["delete_all"] != true {
				t.Fatalf("body = %v", gotBody)
			}
			if _, ok := gotBody["emails"]; ok {
				t.Fatalf("emails sent alongside delete_all: %v", gotBody)
			}
		})
	}
}

func TestSuppressionsDeleteModeValidation(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(204)
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			// Emails and Delete All are mutually exclusive.
			out, err := tc.del(nil, nil, []*core.Connection{
				secretConn("api_key", "SG.testkey"),
				strConn("emails", "a@x.com"),
				boolConn("delete_all", true),
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), "not both") {
				t.Fatalf("both-modes result = %v", out)
			}
			// At least one mode is required.
			out, err = tc.del(nil, nil, []*core.Connection{secretConn("api_key", "SG.testkey")})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false {
				t.Fatalf("no-mode result = %v", out)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("request sent despite validation failure (%d calls)", calls)
	}
}

func TestSuppressionsDeleteAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"errors":[{"message":"something went wrong","field":null}]}`))
	}))
	defer srv.Close()
	defer sendgrid.SetBaseForTest(srv.URL)()

	for _, tc := range suppressionResources {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.del(nil, nil, []*core.Connection{
				secretConn("api_key", "SG.testkey"),
				strConn("email", "jane@example.com"),
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), "something went wrong") {
				t.Fatalf("API error result = %v", out)
			}
		})
	}
}
