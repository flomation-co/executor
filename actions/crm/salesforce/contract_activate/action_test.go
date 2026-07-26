package crm_salesforce_contract_activate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const contractID = "800aj000039RDKDAA4"

// org stands in for Salesforce: it accepts the status PATCH (which Salesforce
// really does for every one of Contract.Status's three values — verified live,
// 204 for Draft and In Approval Process as well as Activated) and answers the
// read-back with whatever state the test wants.
type org struct {
	status     string
	statusCode string
	patched    map[string]interface{}
}

func (o *org) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/sobjects/Contract/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodPatch:
			_ = json.NewDecoder(r.Body).Decode(&o.patched)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":             contractID,
				"ContractNumber": "00000120",
				"Status":         o.status,
				"StatusCode":     o.statusCode,
				"StartDate":      "2026-08-01",
				"EndDate":        "2027-07-31",
				"ContractTerm":   12,
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	restore := salesforce.SetHostForTest(srv.URL)
	return func() {
		restore()
		srv.Close()
	}
}

func auth() []*core.Connection {
	return []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "contract_id", Type: core.ConnectionTypeString, Value: contractID},
	}
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// TestANonActivatingStatusIsNotReportedAsAnActivation is the regression test.
// The Status To Set box is backed by the live Contract.Status picklist, which
// offers In Approval Process, Activated and Draft — in that order, so the
// NON-activating value is the first row an operator sees. Salesforce accepts all
// three with a 204, and the summary used to read "Activated contract 00000120 —
// its status is now \"Draft\"" on the SUCCESS port, with ActivatedDate null:
// every downstream step gated on "Activate Contract succeeded" (invoice, welcome
// email, onboarding task) then fired against a draft.
func TestANonActivatingStatusIsNotReportedAsAnActivation(t *testing.T) {
	for _, tc := range []struct {
		status     string
		statusCode string
	}{
		{"Draft", "Draft"},
		{"In Approval Process", "InApproval"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			o := &org{status: tc.status, statusCode: tc.statusCode}
			defer o.serve(t)()

			out, err := Execute(&core.Flow{}, &core.Node{}, append(auth(), str("contract_status", tc.status)))
			if err != nil {
				t.Fatalf("unexpected hard error: %v", err)
			}
			if ok, _ := out["success"].(bool); ok {
				t.Fatalf("a contract left in %q must NOT be reported as a success: %v", tc.status, out["tool_result"])
			}
			msg, _ := out["tool_result"].(string)
			if strings.Contains(msg, "Activated contract") {
				t.Errorf("the summary still claims an activation that did not happen: %q", msg)
			}
			if !strings.Contains(msg, "has NOT been activated") {
				t.Errorf("the operator has to be told the contract is not live, got: %q", msg)
			}
			if !strings.Contains(msg, tc.status) {
				t.Errorf("the summary should name the status that was actually set, got: %q", msg)
			}
		})
	}
}

// The ordinary path — Status To Set left blank — is unchanged: Activated goes in,
// StatusCode comes back Activated, and the summary reports the activation and the
// end date.
func TestBlankStatusActivatesAndReportsIt(t *testing.T) {
	o := &org{status: "Activated", statusCode: "Activated"}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, auth())
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}
	if got := o.patched["Status"]; got != "Activated" {
		t.Errorf("a blank Status To Set must send Activated, sent %v", got)
	}
	msg, _ := out["tool_result"].(string)
	if !strings.Contains(msg, "Activated contract 00000120") || !strings.Contains(msg, "2027-07-31") {
		t.Errorf("unexpected summary: %q", msg)
	}
}

// An org that RENAMED its live status is the reason the override exists, and it
// still works: Status reads back as the org's own word for live while StatusCode
// — which Salesforce sets itself — reads Activated.
func TestARenamedLiveStatusStillCountsAsActivated(t *testing.T) {
	o := &org{status: "Contract Live", statusCode: "Activated"}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, append(auth(), str("contract_status", "Contract Live")))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("a renamed live status must still succeed, got %v", out["error"])
	}
	if msg, _ := out["tool_result"].(string); !strings.Contains(msg, "Activated contract") {
		t.Errorf("unexpected summary: %q", msg)
	}
}
