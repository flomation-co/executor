package crm_salesforce_quote_line_item_create

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// The regression: with no price book to narrow by, the price book entry was
// chosen with LIMIT 1 and no ORDER BY, so Salesforce picked the row. Live, a
// product listed at 25,000 on Standard and 1.00 on a Wholesale book returned the
// 1.00 row: the quote line was written at 1.00 AND the quote was then pinned to
// the wholesale book, so every later line followed it. A CRM must not invent a
// price — it has to say which books disagree.

// fakeOrg answers the four requests this action can make, and records the writes
// so a test can prove that a refusal wrote nothing at all.
type fakeOrg struct {
	entries []map[string]interface{} // what the PricebookEntry query returns
	writes  []string
}

func (f *fakeOrg) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		soql := r.URL.Query().Get("q")
		switch {
		case strings.Contains(soql, "FROM Quote"):
			// A quote with no price book and no parent deal — the shape an
			// operator gets from Create Quote with the optional book left blank.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 1, "done": true,
				"records": []map[string]interface{}{{"Id": "0Q05f000000AbCdAAK", "Pricebook2Id": nil, "OpportunityId": nil}},
			})
		case strings.Contains(soql, "FROM PricebookEntry"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": len(f.entries), "done": true, "records": f.entries,
			})
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/sobjects/Quote/"):
			f.writes = append(f.writes, "PATCH Quote")
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/sobjects/QuoteLineItem"):
			f.writes = append(f.writes, "POST QuoteLineItem")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "0QL5f000003AbCdGAK", "success": true, "errors": []interface{}{},
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

func entry(id, book, bookName string, price float64) map[string]interface{} {
	return map[string]interface{}{
		"Id": id, "UnitPrice": price, "Product2Id": "01t5f000004AbCdAAK", "Pricebook2Id": book,
		"Pricebook2": map[string]interface{}{"Name": bookName},
	}
}

func inputs(extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "quote_id", Type: core.ConnectionTypeString, Value: "0Q05f000000AbCdAAK"},
		{Name: "product_id", Type: core.ConnectionTypeString, Value: "01t5f000004AbCdAAK"},
	}, extra...)
}

func TestTwoActivePriceBooksIsRefusedAndNamesBoth(t *testing.T) {
	org := &fakeOrg{entries: []map[string]interface{}{
		entry("01u5f000000WholAAC", "01s5f00000WholAAF", "Wholesale", 1),
		entry("01u5f000000StndAAC", "01s5f00000StndAAF", "Standard", 25000),
	}}
	defer org.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs())
	if err != nil {
		t.Fatalf("this is a provider-shaped outcome and belongs on the error port, got a hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); ok {
		t.Fatalf("a product priced on two active books must not be guessed at: %v", out["tool_result"])
	}
	msg, _ := out["error"].(string)
	for _, want := range []string{"Wholesale", "Standard", "1.00", "25000.00"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal must name each book and its price (%q missing): %q", want, msg)
		}
	}
	// The old code pinned the quote to the guessed book and then wrote the line.
	if len(org.writes) != 0 {
		t.Errorf("a refused line must leave the quote untouched, got writes %v", org.writes)
	}
}

// The ordinary one-book org must be unaffected: the price is still found for the
// operator and the quote is still pinned so the next line works.
func TestSingleActivePriceBookStillPricesTheLine(t *testing.T) {
	org := &fakeOrg{entries: []map[string]interface{}{
		entry("01u5f000000StndAAC", "01s5f00000StndAAF", "Standard", 25000),
	}}
	defer org.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs())
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}
	if len(org.writes) != 2 || org.writes[0] != "PATCH Quote" || org.writes[1] != "POST QuoteLineItem" {
		t.Errorf("expected the quote to be pinned and the line written, got %v", org.writes)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "Added 1 x product line") {
		t.Errorf("unexpected summary: %q", summary)
	}
}

// Pinning is reported to a non-technical operator, so the summary has to name the
// price book. It used to print Pricebook2Id — an 18-character record ID that says
// nothing to the person reading the run, even though the query already selects
// Pricebook2.Name.
func TestPinningSummaryNamesThePriceBookNotItsID(t *testing.T) {
	org := &fakeOrg{entries: []map[string]interface{}{
		entry("01u5f000000StndAAC", "01s5f00000StndAAF", "Standard Price Book", 25000),
	}}
	defer org.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs())
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	summary, _ := out["tool_result"].(string)
	if !strings.Contains(summary, "Standard Price Book") {
		t.Errorf("the summary must name the price book the quote was pinned to: %q", summary)
	}
	if strings.Contains(summary, "01s5f00000StndAAF") {
		t.Errorf("the summary still prints the raw price book ID at the operator: %q", summary)
	}
}
