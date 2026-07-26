package crm_salesforce_quote_create

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

func acceptCreate(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/sobjects/Quote") {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "0Q0aj000002vJ3BCAU", "success": true})
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
		{Name: "name", Type: core.ConnectionTypeString, Value: "Acme Ltd - 50 seat renewal"},
	}, extra...)
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// TestThePriceBookNoteIsNotStatedAsAFactWhenADealCanSupplyOne is the regression
// test. The note was gated on the Price Book INPUT being blank, not on the created
// record — but a quote raised on a deal INHERITS the deal's price book at insert
// (verified live: the quote came back on the opportunity's own book and took a
// product line straight away). Told flatly "no price book was set, so add one
// before adding products", an operator goes looking for a step they do not need —
// or sets a different book with Update Quote, which is how a quote ends up priced
// off the wrong list, since Salesforce does not require the two to match.
func TestThePriceBookNoteIsNotStatedAsAFactWhenADealCanSupplyOne(t *testing.T) {
	defer acceptCreate(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs(str("opportunity_id", "006aj00000ZwpAzAAJ")))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	summary, _ := out["tool_result"].(string)
	if strings.Contains(summary, "no price book was set, so add one") {
		t.Errorf("the note is stated as a fact about a quote that has probably inherited the deal's price book: %q", summary)
	}
	if !strings.Contains(summary, "the deal's own price book") {
		t.Errorf("the note should say the quote takes the deal's price book, got: %q", summary)
	}
}

// With no deal to inherit from the note is correct, and stays as it was.
func TestThePriceBookNoteStandsWhenThereIsNoDeal(t *testing.T) {
	defer acceptCreate(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs())
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	summary, _ := out["tool_result"].(string)
	if !strings.Contains(summary, "no price book was set, so add one before adding products") {
		t.Errorf("a standalone quote with no price book still needs the warning, got: %q", summary)
	}
}

// A quote created with its own price book gets no note at all.
func TestNoNoteWhenAPriceBookWasGiven(t *testing.T) {
	defer acceptCreate(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs(str("pricebook_id", "01saj00000LXMP3AAP")))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if summary, _ := out["tool_result"].(string); strings.Contains(summary, "price book") {
		t.Errorf("no price-book note is needed here, got: %q", summary)
	}
}
