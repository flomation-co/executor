package crm_salesforce_opportunity_upsert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

type upsertRecorder struct {
	body map[string]interface{}
}

func (u *upsertRecorder) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/sobjects/Opportunity/Order_Reference__c/") {
			raw, _ := io.ReadAll(r.Body)
			u.body = map[string]interface{}{}
			_ = json.Unmarshal(raw, &u.body)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "0065f00000AbCdEAAV", "success": true, "created": true, "errors": []interface{}{},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	restore := salesforce.SetHostForTest(srv.URL)
	return func() {
		restore()
		srv.Close()
	}
}

func base() []*core.Connection {
	return []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "external_id_field", Type: core.ConnectionTypeString, Value: "Order_Reference__c"},
		{Name: "external_id_value", Type: core.ConnectionTypeString, Value: "SO-10432"},
	}
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// Record Type was simply missing from this action, so an operator who built
// Create Opportunity with a record type and then swapped in the re-runnable
// version got the profile's default type on every deal, silently.
func TestRecordTypeReachesSalesforce(t *testing.T) {
	rec := &upsertRecorder{}
	defer rec.serve(t)()

	inputs := append(base(),
		str("name", "Acme Ltd - 50 seat renewal"),
		str("stage_name", "Prospecting"),
		str("record_type_id", "0125f000000AbCdAAK"),
	)
	if _, err := Execute(&core.Flow{}, &core.Node{}, inputs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.body["RecordTypeId"] != "0125f000000AbCdAAK" {
		t.Errorf("RecordTypeId was dropped from the payload: %v", rec.body)
	}

	// And it stays opt-in: an untouched box must not send an empty type, which
	// would blank the record type on a matched deal.
	rec.body = nil
	if _, err := Execute(&core.Flow{}, &core.Node{}, append(base(), str("stage_name", "Prospecting"))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := rec.body["RecordTypeId"]; present {
		t.Errorf("an untouched Record Type must not be written: %v", rec.body)
	}
}

func TestRecordTypeInputExists(t *testing.T) {
	for i := range Inputs {
		if Inputs[i].Name == "record_type_id" {
			return
		}
	}
	t.Fatal("record_type_id input is missing — the editor cannot supply a record type at all without it")
}

// A mistyped Match On Field ("Order Reference__c", with a space — the label
// rather than the API name) is a configuration mistake no retry can fix. It
// used to reach the soft error port, where the flow carried on down its failure
// branch as though Salesforce had refused a well-formed request; record_upsert
// and account_upsert hard-fail on the identical value.
func TestMistypedMatchFieldHardFails(t *testing.T) {
	rec := &upsertRecorder{}
	defer rec.serve(t)()

	inputs := []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "external_id_field", Type: core.ConnectionTypeString, Value: "Order Reference__c"},
		{Name: "external_id_value", Type: core.ConnectionTypeString, Value: "SO-10432"},
	}
	out, err := Execute(&core.Flow{}, &core.Node{}, inputs)
	if err == nil {
		t.Fatalf("expected a hard error, got %v", out)
	}
	if out != nil {
		t.Error("a configuration mistake must return a nil result, not error-port data")
	}
	if !strings.Contains(err.Error(), "Match On Field") {
		t.Errorf("the error should name the input the operator has to fix, got %q", err)
	}
	if rec.body != nil {
		t.Error("nothing should have been sent to Salesforce")
	}
}
