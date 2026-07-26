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

const (
	testOrderID = "801aj00003DvqyXAAR"
	testEntryID = "01uaj000008Qi5yAAC"
	testBookID  = "01saj00000LXMP3AAP"
	otherBookID = "01saj00000LXMP4AAP"
)

// entryOrg stands in for Salesforce on the OPERATOR-SUPPLIED-ENTRY path: a draft
// order already on a price book, plus one entry read back on whichever book the
// test names. It records writes so a refusal can be shown to have written nothing.
type entryOrg struct {
	entryBook string
	created   map[string]interface{}
	writes    []string
}

func (o *entryOrg) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		soql := r.URL.Query().Get("q")
		switch {
		case strings.Contains(soql, "FROM Order"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 1, "done": true,
				"records": []map[string]interface{}{{
					"Id": testOrderID, "Pricebook2Id": testBookID, "Status": "Draft",
					"EffectiveDate": "2026-01-01", "EndDate": nil,
				}},
			})
		case strings.Contains(soql, "FROM PricebookEntry"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 1, "done": true,
				"records": []map[string]interface{}{{
					"Id": testEntryID, "UnitPrice": 25000.0, "Product2Id": "01t5f000004AbCdAAK",
					"Pricebook2Id": o.entryBook,
				}},
			})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/sobjects/OrderItem"):
			o.writes = append(o.writes, "POST OrderItem")
			_ = json.NewDecoder(r.Body).Decode(&o.created)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "802aj00000aOrw5AAC", "success": true})
		case r.Method == http.MethodPatch:
			o.writes = append(o.writes, "PATCH Order")
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

func entryInputs(extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "order_id", Type: core.ConnectionTypeString, Value: testOrderID},
		{Name: "pricebook_entry_id", Type: core.ConnectionTypeString, Value: testEntryID},
	}, extra...)
}

func strInput(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// TestACrossBookEntryIsAnErrorPortOutcomeNotANodeFailure is the regression test.
// A price book entry priced on a different book from the order is a PROVIDER state,
// only discoverable by reading both records back — and the identical branch on Add
// Product to Quote hands it back as a soft failure. Here it was a hard error, so
// the node failed outright, the flow's own "tell the sales admin" branch never ran
// and the run stopped: two halves of one commerce flow behaving differently on the
// same Salesforce outcome.
func TestACrossBookEntryIsAnErrorPortOutcomeNotANodeFailure(t *testing.T) {
	o := &entryOrg{entryBook: otherBookID}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, entryInputs())
	if err != nil {
		t.Fatalf("a price-book mismatch must reach the error port, not take the node down: %v", err)
	}
	if out == nil {
		t.Fatal("no outputs were returned, so no error branch can run")
	}
	if ok, _ := out["success"].(bool); ok {
		t.Fatalf("a cross-book entry cannot be added, got success: %v", out["tool_result"])
	}
	msg, _ := out["error"].(string)
	for _, want := range []string{otherBookID, testBookID, "price book"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the message must name both books (%q missing): %q", want, msg)
		}
	}
	if len(o.writes) != 0 {
		t.Errorf("a refused line must leave the order untouched, got writes %v", o.writes)
	}
}

// TestAnExplicitZeroQuantityIsRefusedNotRewrittenToOne: Salesforce refuses a zero
// quantity on an order line outright ("Can't save order products with quantities
// of zero"), so folding an explicit 0 in with "not filled in" put a line the
// source data never asked for on the order a warehouse picks from — reported as
// "Added 1 x product line" on the success port.
func TestAnExplicitZeroQuantityIsRefusedNotRewrittenToOne(t *testing.T) {
	o := &entryOrg{entryBook: testBookID}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, entryInputs(strInput("quantity", "0")))
	if err == nil {
		t.Fatalf("a quantity of 0 must be refused, got: %v", out)
	}
	if !strings.Contains(err.Error(), "0") || !strings.Contains(err.Error(), "blank") {
		t.Errorf("the message should name the zero and the way out, got: %v", err)
	}
	if len(o.writes) != 0 {
		t.Errorf("nothing may be written for a refused quantity, got %v", o.writes)
	}
}

// Leaving the box blank still means one, and a real figure still goes through.
func TestBlankAndRealQuantitiesAreUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name  string
		given []*core.Connection
		want  float64
	}{
		{"blank", nil, 1},
		{"three", []*core.Connection{strInput("quantity", "3")}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := &entryOrg{entryBook: testBookID}
			defer o.serve(t)()

			out, err := Execute(&core.Flow{}, &core.Node{}, entryInputs(tc.given...))
			if err != nil {
				t.Fatalf("unexpected hard error: %v", err)
			}
			if ok, _ := out["success"].(bool); !ok {
				t.Fatalf("expected success, got %v", out["error"])
			}
			if got := o.created["Quantity"]; got != tc.want {
				t.Errorf("expected Quantity %v, sent %v", tc.want, got)
			}
		})
	}
}
