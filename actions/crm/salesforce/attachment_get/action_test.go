package crm_salesforce_attachment_get

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

func serve(t *testing.T, called *bool) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Id": "00P5f00000XyzAAAAA", "Name": "quote.pdf", "ContentType": "application/pdf",
		})
	}))
	restore := salesforce.SetHostForTest(srv.URL)
	return func() {
		restore()
		srv.Close()
	}
}

func inputs(fields string) []*core.Connection {
	return []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "attachment_id", Type: core.ConnectionTypeString, Value: "00P5f00000XyzAAAAA"},
		{Name: "fields", Type: core.ConnectionTypeString, Value: fields},
	}
}

// Same split as opportunity_get and lead_get: a misspelled field name never
// reaches Salesforce, so it is a configuration mistake and hard-fails rather
// than arriving on the error port dressed up as a provider failure.
func TestInvalidFieldNameHardFails(t *testing.T) {
	called := false
	defer serve(t, &called)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs("Name, Content Type"))
	if err == nil {
		t.Fatalf("expected a hard error, got %v", out)
	}
	if out != nil {
		t.Error("a configuration mistake must return a nil result, not error-port data")
	}
	if !strings.Contains(err.Error(), "Fields") {
		t.Errorf("the error should name the input to fix, got %q", err)
	}
	if called {
		t.Error("the call should never have been sent to Salesforce")
	}
}

func TestValidFieldListStillWorks(t *testing.T) {
	called := false
	defer serve(t, &called)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs("Name, ContentType"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}
	if !called {
		t.Error("a valid field list should have reached Salesforce")
	}
}
