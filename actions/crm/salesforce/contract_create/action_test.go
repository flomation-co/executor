package crm_salesforce_contract_create

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// org records the POST so a test can prove a refused status never reached
// Salesforce, and answers a good create the way Salesforce does.
type org struct {
	created map[string]interface{}
}

func (o *org) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/sobjects/Contract") {
			_ = json.NewDecoder(r.Body).Decode(&o.created)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "800aj000039RDKDAA4", "success": true})
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

func inputs(extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "account_id", Type: core.ConnectionTypeString, Value: "001aj00003CnM29AAF"},
	}, extra...)
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// TestAStatusThatCannotBeCreatedIsRefusedByName is the regression test.
//
// Only Draft is creatable: Salesforce's insert rule is that Status must map to
// StatusCode "Draft", so both other values of the standard picklist are refused
// with FAILED_ACTIVATION ("Choose a valid contract status and save your changes"),
// verified live. The Status box's live dropdown offers all three with In Approval
// Process FIRST, and the old placeholder actively recommended it — after which the
// FAILED_ACTIVATION translation talked only about Activated, so the advice read as
// though it were about a different field.
func TestAStatusThatCannotBeCreatedIsRefusedByName(t *testing.T) {
	for _, tc := range []struct {
		status string
		names  []string
	}{
		{"In Approval Process", []string{"In Approval Process", "Draft", "Update Contract"}},
		{"Activated", []string{"Activated", "Draft", "Activate Contract"}},
		// The dropdown sends Salesforce's own spelling, but a variable or a hand-typed
		// value can arrive in any case.
		{"activated", []string{"Activated", "Draft"}},
	} {
		t.Run(tc.status, func(t *testing.T) {
			o := &org{}
			defer o.serve(t)()

			out, err := Execute(&core.Flow{}, &core.Node{}, inputs(str("contract_status", tc.status)))
			if err == nil {
				t.Fatalf("%q cannot be created and must be refused, got: %v", tc.status, out)
			}
			for _, want := range tc.names {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the message should mention %q, got: %v", want, err)
				}
			}
			if o.created != nil {
				t.Errorf("the contract was posted anyway: %v", o.created)
			}
		})
	}
}

// The documented path — Status left blank — is unchanged, and Draft is still
// accepted for an operator who names it explicitly.
func TestDraftAndBlankStillCreate(t *testing.T) {
	for _, status := range []string{"", "Draft"} {
		o := &org{}
		restore := o.serve(t)

		given := inputs()
		if status != "" {
			given = inputs(str("contract_status", status))
		}
		out, err := Execute(&core.Flow{}, &core.Node{}, given)
		restore()
		if err != nil {
			t.Fatalf("status %q must still create: %v", status, err)
		}
		if ok, _ := out["success"].(bool); !ok {
			t.Fatalf("status %q must still create, got %v", status, out["error"])
		}
		if o.created == nil {
			t.Fatalf("status %q did not reach Salesforce", status)
		}
		if got, ok := o.created["Status"]; status == "" && ok {
			t.Errorf("a blank Status must be omitted so Salesforce applies its own default, sent %v", got)
		}
	}
}

// TestTheStatusPlaceholderDoesNotRecommendAnUncreatableValue: the placeholder is
// the only guidance an operator gets on this box, and it used to name In Approval
// Process as an acceptable create value.
func TestTheStatusPlaceholderDoesNotRecommendAnUncreatableValue(t *testing.T) {
	for i := range Inputs {
		if Inputs[i].Name != "contract_status" {
			continue
		}
		p := Inputs[i].Placeholder
		if strings.Contains(p, "In Approval Process") {
			t.Errorf("the placeholder still offers a value Salesforce always refuses on create: %q", p)
		}
		if !strings.Contains(p, "Draft") {
			t.Errorf("the placeholder should say a new contract starts as a Draft, got %q", p)
		}
		return
	}
	t.Fatal("contract_status input is missing")
}
