package crm_salesforce_opportunity_get

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// serve stands in for the org and records whether it was called at all — a
// configuration mistake must never reach Salesforce.
func serve(t *testing.T, called *bool) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"Id": "0065f00000AbCdEAAV", "Name": "Acme renewal"})
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
		{Name: "opportunity_id", Type: core.ConnectionTypeString, Value: "0065f00000AbCdEAAV"},
		{Name: "fields", Type: core.ConnectionTypeString, Value: fields},
	}
}

// Typing the on-screen label instead of the API name — "Close Date" with the
// space — is the commonest Salesforce mistake there is. It used to land on the
// ERROR PORT as data, so the flow ran on down its failure branch as though
// Salesforce had refused the call and any retry wired there re-ran a typo
// forever. account_get and case_get hard-fail on the identical input.
func TestInvalidFieldNameHardFails(t *testing.T) {
	called := false
	defer serve(t, &called)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs("Amount, Close Date"))
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

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs("Amount, CloseDate"))
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
