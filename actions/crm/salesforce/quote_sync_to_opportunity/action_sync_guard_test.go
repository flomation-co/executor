package crm_salesforce_quote_sync_to_opportunity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// The regression: a sync REPLACES the deal's product lines with the quote's, so
// syncing a quote that has no lines DELETED the deal's lines and zeroed its
// Amount — Salesforce answered 204 and the run reported plain success. Verified
// live: a 50,000 deal with one line came back Amount 0.0 with no lines, and
// Create Quote does not copy the deal's products, so the obvious two-node flow
// destroyed the forecast.

// org answers the two reads and the two counts, and records whether the
// destructive PATCH was issued.
type org struct {
	quoteLines int
	oppLines   int
	patched    bool
	// standalone makes the quote's OpportunityId null, which is an ordinary
	// record wherever "Create Quotes Without a Related Opportunity" is on.
	standalone bool
}

func (o *org) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		soql := r.URL.Query().Get("q")
		switch {
		case strings.Contains(soql, "COUNT() FROM QuoteLineItem"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": o.quoteLines, "done": true, "records": []map[string]interface{}{},
			})
		case strings.Contains(soql, "COUNT() FROM OpportunityLineItem"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": o.oppLines, "done": true, "records": []map[string]interface{}{},
			})
		case strings.Contains(soql, "FROM Quote"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 1, "done": true,
				"records": []map[string]interface{}{{
					"Id": "0Q05f000000AbCdAAK", "Name": "Q-1", "QuoteNumber": "00000123",
					"OpportunityId": func() interface{} {
						if o.standalone {
							return nil
						}
						return "0065f00000AbCdEAAV"
					}(),
				}},
			})
		case strings.Contains(soql, "FROM Opportunity"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 1, "done": true,
				"records": []map[string]interface{}{{"Id": "0065f00000AbCdEAAV", "SyncedQuoteId": nil}},
			})
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/sobjects/Opportunity/"):
			o.patched = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	restore := salesforce.SetHostForTest(srv.URL)
	return func() {
		restore()
		srv.Close()
	}
}

func inputs() []*core.Connection {
	return []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "quote_id", Type: core.ConnectionTypeString, Value: "0Q05f000000AbCdAAK"},
	}
}

func TestSyncingALinelessQuoteOntoADealWithLinesIsRefused(t *testing.T) {
	o := &org{quoteLines: 0, oppLines: 1}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs())
	if err != nil {
		t.Fatalf("this is a provider-shaped outcome and belongs on the error port, got: %v", err)
	}
	if ok, _ := out["success"].(bool); ok {
		t.Fatalf("wiping the deal's lines must not be reported as success: %v", out["tool_result"])
	}
	msg, _ := out["error"].(string)
	for _, want := range []string{"no product lines", "DELETE", "Add Product to Quote"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal must say what would be lost and what to do (%q missing): %q", want, msg)
		}
	}
	// The critical assertion: the deal must not be touched at all.
	if o.patched {
		t.Error("SyncedQuoteId was written even though the sync would have deleted the deal's lines")
	}
}

// A quote that HAS lines still syncs, and the summary has to record the
// replacement — the execution log is the only trace an operator ever gets.
func TestSyncingAQuoteWithLinesSaysWhatItReplaced(t *testing.T) {
	o := &org{quoteLines: 3, oppLines: 1}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs())
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}
	if !o.patched {
		t.Error("the sync should have been written")
	}
	summary, _ := out["tool_result"].(string)
	if !strings.Contains(summary, "REPLACED the deal's 1 product line(s) with the quote's 3") {
		t.Errorf("the summary must state what the sync overwrote, got %q", summary)
	}
}

// An empty deal is not at risk, so a lineless quote there is allowed through —
// the guard must not turn a harmless sync into a failure.
func TestSyncingALinelessQuoteOntoAnEmptyDealStillWorks(t *testing.T) {
	o := &org{quoteLines: 0, oppLines: 0}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs())
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("nothing is destroyed here, so this must still sync: %v", out["error"])
	}
	if !o.patched {
		t.Error("the sync should have been written")
	}
}

// A standalone quote synced to a deal chosen in the Opportunity box used to walk
// straight into the write, and Salesforce answered
// INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY — which reads as a permissions
// problem. The operator had no permissions problem: Salesforce only syncs a
// quote that is a CHILD of the deal. Worse, the action's own message for the
// no-deal case invited them to "name the deal here", so the node sent them down
// the failing path itself.
func TestSyncingAStandaloneQuoteToAChosenDealNamesTheRuleNotPermissions(t *testing.T) {
	o := &org{quoteLines: 2, oppLines: 0, standalone: true}
	defer o.serve(t)()

	in := append(inputs(), &core.Connection{
		Name: "opportunity_id", Type: core.ConnectionTypeString, Value: "0065f00000AbCdEAAV",
	})
	out, err := Execute(&core.Flow{}, &core.Node{}, in)
	if err == nil {
		if ok, _ := out["success"].(bool); ok {
			t.Fatalf("a quote that is not the deal's child cannot sync, so this must not report success: %v", out["tool_result"])
		}
	}
	msg, _ := out["error"].(string)
	if msg == "" && err != nil {
		msg = err.Error()
	}
	if strings.Contains(strings.ToLower(msg), "insufficient access") ||
		strings.Contains(strings.ToLower(msg), "cannot see it") {
		t.Errorf("this is not a permissions problem and must not be described as one: %q", msg)
	}
	for _, want := range []string{"not attached", "Update Quote"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal must name the parent-child rule and the one step that fixes it (%q missing): %q", want, msg)
		}
	}
	if o.patched {
		t.Error("the deal was written even though Salesforce cannot sync a quote that is not its child")
	}
}

// Stopping a sync only clears a field, so it never needs the quote to be the
// deal's child — the new guard must not block it.
func TestStopSyncingIsNotBlockedByTheParentChildGuard(t *testing.T) {
	o := &org{quoteLines: 0, oppLines: 3, standalone: true}
	defer o.serve(t)()

	in := append(inputs(),
		&core.Connection{Name: "opportunity_id", Type: core.ConnectionTypeString, Value: "0065f00000AbCdEAAV"},
		&core.Connection{Name: "stop_syncing", Type: core.ConnectionTypeBoolean, Value: true},
	)
	out, err := Execute(&core.Flow{}, &core.Node{}, in)
	if err != nil {
		t.Fatalf("stopping a sync must not be refused: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Errorf("stopping a sync only clears a field and must be allowed: %v", out["error"])
	}
}
