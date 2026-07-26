package crm_salesforce_price_book_entry_create

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// The summary used to print two 18-character record IDs — "Priced product
// 01taj00000UR0M1AAL at 1234.56 on price book 01saj00000LXMP4AAP" — at whoever
// reads the run. The product in particular is usually NOT something the operator
// picked on this node: the canonical flow is Create or Update Product followed by
// this action with Product bound to the upstream node's id, so nobody has ever
// seen the product's name here.

// org answers the create and the label read-back, and records whether the
// read-back was attempted at all.
type org struct {
	labelStatus int  // status to answer the PricebookEntry query with
	asked       bool // was the label query issued?
}

func (o *org) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/sobjects/PricebookEntry"):
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "01uaj000008VBNZAA4", "success": true, "errors": []interface{}{},
			})
		case strings.Contains(r.URL.Query().Get("q"), "FROM PricebookEntry"):
			o.asked = true
			if o.labelStatus != http.StatusOK {
				w.WriteHeader(o.labelStatus)
				// Salesforce's error envelope is a JSON ARRAY.
				_ = json.NewEncoder(w).Encode([]map[string]interface{}{
					{"message": "no", "errorCode": "INVALID_SESSION_ID"},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 1, "done": true,
				"records": []map[string]interface{}{{
					"Id":         "01uaj000008VBNZAA4",
					"Product2":   map[string]interface{}{"Name": "GenWatt Diesel 200kW"},
					"Pricebook2": map[string]interface{}{"Name": "Standard Price Book"},
				}},
			})
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

func inputs(extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "pricebook_id", Type: core.ConnectionTypeString, Value: "01saj00000LXMP4AAP"},
		{Name: "product_id", Type: core.ConnectionTypeString, Value: "01taj00000UR0M1AAL"},
		{Name: "unit_price", Type: core.ConnectionTypeString, Value: "1234.56"},
	}, extra...)
}

func TestSummaryNamesTheProductAndTheBook(t *testing.T) {
	o := &org{labelStatus: http.StatusOK}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs())
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}
	summary, _ := out["tool_result"].(string)
	for _, want := range []string{"GenWatt Diesel 200kW", "Standard Price Book", "1234.56"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary must name %q: %q", want, summary)
		}
	}
	for _, unwanted := range []string{"01taj00000UR0M1AAL", "01saj00000LXMP4AAP"} {
		if strings.Contains(summary, unwanted) {
			t.Errorf("summary still prints the raw ID %s at the operator: %q", unwanted, summary)
		}
	}
}

// THE case that matters. The label read-back is cosmetic and runs AFTER the write
// has already succeeded, so a failure there must not turn a completed write into
// an error — reporting failure would send the operator to create the price a
// second time, and Salesforce allows only one per product per book, so the retry
// fails too and the run looks broken when nothing is.
func TestAFailedLabelLookupStillReportsTheSuccessfulWrite(t *testing.T) {
	o := &org{labelStatus: http.StatusUnauthorized}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs())
	if err != nil {
		t.Fatalf("a cosmetic read must never produce a hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("the price WAS created, so this must still report success: %v", out["error"])
	}
	if !o.asked {
		t.Error("the label lookup was never attempted, so this test proves nothing about failing open")
	}
	if id, _ := out["id"].(string); id != "01uaj000008VBNZAA4" {
		t.Errorf("the created id must still be returned, got %q", id)
	}
	// Falls back to the exact previous wording rather than a half-finished sentence.
	summary, _ := out["tool_result"].(string)
	for _, want := range []string{"product 01taj00000UR0M1AAL", "01saj00000LXMP4AAP", "1234.56"} {
		if !strings.Contains(summary, want) {
			t.Errorf("the fallback must read as it always did (%q missing): %q", want, summary)
		}
	}
}

// With Copy The Standard Price on, Salesforce overwrites the figure that was
// sent, so the summary must not quote the operator's number back — but it should
// still name the product rather than print its ID.
func TestStandardPriceSummaryNamesTheProductAndOmitsThePrice(t *testing.T) {
	o := &org{labelStatus: http.StatusOK}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs(
		&core.Connection{Name: "use_standard_price", Type: core.ConnectionTypeBoolean, Value: true},
	))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	summary, _ := out["tool_result"].(string)
	if !strings.Contains(summary, "GenWatt Diesel 200kW") {
		t.Errorf("summary must name the product: %q", summary)
	}
	if !strings.Contains(summary, "standard list price") {
		t.Errorf("summary must say the list price was used: %q", summary)
	}
	if strings.Contains(summary, "1234.56") {
		t.Errorf("Salesforce overwrites the figure sent, so quoting it back names a price the record does not hold: %q", summary)
	}
}
