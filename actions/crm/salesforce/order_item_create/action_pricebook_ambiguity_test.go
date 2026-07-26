package crm_salesforce_order_item_create

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// The order half of the same regression, and the worse half: an order created
// through the API has NO price book of its own (verified live, Pricebook2Id comes
// back null), so the widened "any active entry on any active book" search — and
// the LIMIT 1 with no ORDER BY that used to resolve it — was the DEFAULT path for
// every order, not an edge case. The line was written at whichever price came
// back and the order was then pinned to that book.

type fakeOrg struct {
	entries []map[string]interface{}
	writes  []string
}

func (f *fakeOrg) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		soql := r.URL.Query().Get("q")
		switch {
		case strings.Contains(soql, "FROM Order"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 1, "done": true,
				"records": []map[string]interface{}{{
					"Id": "8015f00000AbCdEAAV", "Pricebook2Id": nil, "Status": "Draft",
					"EffectiveDate": "2026-01-01", "EndDate": nil,
				}},
			})
		case strings.Contains(soql, "FROM PricebookEntry"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": len(f.entries), "done": true, "records": f.entries,
			})
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/sobjects/Order/"):
			f.writes = append(f.writes, "PATCH Order")
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/sobjects/OrderItem"):
			f.writes = append(f.writes, "POST OrderItem")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "8025f00000AbCdEAAV", "success": true, "errors": []interface{}{},
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

func inputs() []*core.Connection {
	return []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "order_id", Type: core.ConnectionTypeString, Value: "8015f00000AbCdEAAV"},
		{Name: "product_id", Type: core.ConnectionTypeString, Value: "01t5f000004AbCdAAK"},
	}
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
	for _, want := range []string{"Wholesale", "Standard", "1.00", "25000.00", "order"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal must name each book and its price (%q missing): %q", want, msg)
		}
	}
	if len(org.writes) != 0 {
		t.Errorf("a refused line must leave the order untouched, got writes %v", org.writes)
	}
}

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
	if len(org.writes) != 2 || org.writes[0] != "PATCH Order" || org.writes[1] != "POST OrderItem" {
		t.Errorf("expected the order to be pinned and the line written, got %v", org.writes)
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
		t.Errorf("the summary must name the price book the order was pinned to: %q", summary)
	}
	if strings.Contains(summary, "01s5f00000StndAAF") {
		t.Errorf("the summary still prints the raw price book ID at the operator: %q", summary)
	}
}
