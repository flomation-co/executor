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

const (
	testQuoteID = "0Q0aj000002vK8vCAE"
	testEntryID = "01uaj000008V6nhAAC"
	testBookID  = "01saj00000LXMP3AAP"
)

// entryOrg stands in for Salesforce on the OPERATOR-SUPPLIED-ENTRY path: the
// quote is already on a price book, and the entry the operator named is read back
// with whatever state the test wants. It records the writes so a test can prove a
// refusal wrote nothing.
type entryOrg struct {
	entryActive bool
	created     map[string]interface{}
	writes      []string
}

func (o *entryOrg) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		soql := r.URL.Query().Get("q")
		switch {
		case strings.Contains(soql, "FROM Quote"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 1, "done": true,
				"records": []map[string]interface{}{{"Id": testQuoteID, "Pricebook2Id": testBookID, "OpportunityId": nil}},
			})
		case strings.Contains(soql, "FROM PricebookEntry"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 1, "done": true,
				"records": []map[string]interface{}{{
					"Id": testEntryID, "UnitPrice": 25000.0, "Product2Id": "01t5f000004AbCdAAK",
					"Pricebook2Id": testBookID, "IsActive": o.entryActive,
				}},
			})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/sobjects/QuoteLineItem"):
			o.writes = append(o.writes, "POST QuoteLineItem")
			_ = json.NewDecoder(r.Body).Decode(&o.created)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "0QLaj000001AbCdAAK", "success": true})
		case r.Method == http.MethodPatch:
			o.writes = append(o.writes, "PATCH Quote")
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
		{Name: "quote_id", Type: core.ConnectionTypeString, Value: testQuoteID},
		{Name: "pricebook_entry_id", Type: core.ConnectionTypeString, Value: testEntryID},
	}, extra...)
}

func strInput(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// TestAnExplicitZeroQuantityIsRefusedNotRewrittenToOne is the regression test. A
// shop or ERP feed that maps a removed line to Quantity 0 used to have its 0
// folded in with "not filled in": the line was written for ONE of the product and
// the run reported "Added 1 x product line" — a £25,000 line the source data never
// asked for, on a customer-facing quote, behind a green success. Salesforce itself
// refuses 0 ("Quantity must be nonzero", verified live), so a refusal is the
// honest outcome and one a flow can act on.
func TestAnExplicitZeroQuantityIsRefusedNotRewrittenToOne(t *testing.T) {
	o := &entryOrg{entryActive: true}
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

// Leaving the box blank still means one — that part was always right, and the
// summary still reports the figure it used.
func TestABlankQuantityStillMeansOne(t *testing.T) {
	o := &entryOrg{entryActive: true}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, entryInputs())
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}
	if got := o.created["Quantity"]; got != 1.0 {
		t.Errorf("a blank Quantity should send 1, sent %v", got)
	}
}

// TestAnInactivePriceIsNamedRatherThanBlamedOnThePriceBook pins the second fix.
// Salesforce DEFAULTS PricebookEntry.IsActive to false and Get Many Price Book
// Entries has "Only Prices That Can Be Used" off by default, so an inactive entry
// ID reaching this box from a list is ordinary. Its book matches the quote's, so
// Salesforce's refusal ("The price book entry is inactive. Ask your Salesforce
// admin for help.") used to be answered with "on a quote line this is almost
// always the price book" — a diagnosis that is flatly wrong here — stacked on top
// of common.go's address State/Province text. It is now caught before the write,
// and the fix named is one this node performs itself.
func TestAnInactivePriceIsNamedRatherThanBlamedOnThePriceBook(t *testing.T) {
	o := &entryOrg{entryActive: false}
	defer o.serve(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, entryInputs())
	if err != nil {
		t.Fatalf("a provider outcome belongs on the error port, got a hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); ok {
		t.Fatalf("an inactive price cannot go on a quote, got success: %v", out["tool_result"])
	}
	msg, _ := out["error"].(string)
	if !strings.Contains(msg, "switched off") || !strings.Contains(msg, "Change Product Price") {
		t.Errorf("the operator has to be told the price is switched off and how to switch it on, got: %q", msg)
	}
	if strings.Contains(msg, "price book") && !strings.Contains(msg, "switched off") {
		t.Errorf("the message must not blame the price book, got: %q", msg)
	}
	if len(o.writes) != 0 {
		t.Errorf("an inactive price must leave the quote untouched, got writes %v", o.writes)
	}
}
