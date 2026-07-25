package crm_salesforce_lead_get

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
			"Id": "00Q5f000004XyzAEAS", "FirstName": "Jane", "LastName": "Smith", "Company": "Acme Ltd",
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
		{Name: "lead_id", Type: core.ConnectionTypeString, Value: "00Q5f000004XyzAEAS"},
		{Name: "fields", Type: core.ConnectionTypeString, Value: fields},
	}
}

// A field label typed in place of the API name is a configuration mistake, and
// this node's own contract (account_get, case_get, record_upsert and 38 other
// files) is that those hard-fail rather than landing on the error port, where
// the flow keeps running down a failure branch retrying something no retry can
// fix.
func TestInvalidFieldNameHardFails(t *testing.T) {
	called := false
	defer serve(t, &called)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs("Email, First Name"))
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

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs("Email, FirstName"))
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
