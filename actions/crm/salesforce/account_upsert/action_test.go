package crm_salesforce_account_upsert

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

// upsertRecorder captures the PATCH body the action sends — the whole question
// here is which fields end up on the wire.
type upsertRecorder struct {
	body map[string]interface{}
}

func (u *upsertRecorder) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/sobjects/Account/External_Ref__c/") {
			raw, _ := io.ReadAll(r.Body)
			u.body = map[string]interface{}{}
			_ = json.Unmarshal(raw, &u.body)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "0015f00000AbCdEAAV", "success": true, "created": false, "errors": []interface{}{},
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
		{Name: "external_id_field", Type: core.ConnectionTypeString, Value: "External_Ref__c"},
		{Name: "external_id_value", Type: core.ConnectionTypeString, Value: "CUST-00142"},
	}
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// The regression: a nightly sync that only refreshes the phone number had to
// fill in Company Name because it was Required, and Name was then written on
// every run — reverting an admin's tidy-up of the account name, silently, with
// the run reporting success. A field-level refresh must be expressible.
func TestPartialRefreshDoesNotRewriteTheName(t *testing.T) {
	rec := &upsertRecorder{}
	defer rec.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, append(base(), str("phone", "+44 20 7946 0958")))
	if err != nil {
		t.Fatalf("Company Name must be optional, got a hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}
	if _, present := rec.body["Name"]; present {
		t.Errorf("Name was written even though the operator left it blank: %v", rec.body)
	}
	if rec.body["Phone"] != "+44 20 7946 0958" {
		t.Errorf("the field the operator DID fill in should be written: %v", rec.body)
	}
	// The summary can no longer quote a name it does not have; it must still
	// say which account was written.
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, `External_Ref__c = "CUST-00142"`) {
		t.Errorf("summary should identify the match, got %q", summary)
	}
}

// Filling the box in still writes it — this is a sync that owns the name.
func TestNameIsWrittenWhenGiven(t *testing.T) {
	rec := &upsertRecorder{}
	defer rec.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, append(base(), str("name", "Acme Manufacturing Ltd")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.body["Name"] != "Acme Manufacturing Ltd" {
		t.Errorf("Name should be written when supplied: %v", rec.body)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, `"Acme Manufacturing Ltd"`) {
		t.Errorf("summary should name the account, got %q", summary)
	}
}

// Company Name must not advertise itself as required any more, or the editor
// will not let the partial-refresh flow above be built at all.
func TestNameInputIsNotRequired(t *testing.T) {
	for i := range Inputs {
		if Inputs[i].Name == "name" {
			if Inputs[i].Required {
				t.Error("name must not be Required — an update-only sync has no business supplying it")
			}
			return
		}
	}
	t.Fatal("name input is missing")
}
