package crm_salesforce_price_book_entry_update

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// refuseWithFieldIntegrity answers every PATCH the way the live org answers a
// reprice of an entry whose UseStandardPrice is on: 400 with a bare
// FIELD_INTEGRITY_EXCEPTION and no useful prose.
func refuseWithFieldIntegrity(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`[{"message":"field integrity exception","errorCode":"FIELD_INTEGRITY_EXCEPTION","fields":[]}]`))
	}))
	restore := salesforce.SetHostForTest(srv.URL)
	return func() {
		restore()
		srv.Close()
	}
}

func priceInputs(extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "pricebook_entry_id", Type: core.ConnectionTypeString, Value: "01uaj000008V6nhAAC"},
	}, extra...)
}

// TestAStandardPriceRefusalIsNotAnAddressLecture is the regression test.
//
// UseStandardPrice lives on the RECORD, not in the request: an entry created with
// "Copy The Standard Price" ticked keeps it ticked, and a later reprice sends
// nothing but UnitPrice. The explanation was gated on the tick box having been
// SENT, so the ordinary reprice fell through to common.go's translation and the
// operator was told to check State/Province and Country against their org's
// address lists — on a PricebookEntry, which has no address fields at all. The
// correct sentence sat unreachable sixteen lines below.
func TestAStandardPriceRefusalIsNotAnAddressLecture(t *testing.T) {
	defer refuseWithFieldIntegrity(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, priceInputs(
		&core.Connection{Name: "unit_price", Type: core.ConnectionTypeString, Value: "750"},
		// The tick box is left untouched, which is the DEFAULT state: the editor
		// sends a nil value and SetBoolIfSet omits the field entirely.
		&core.Connection{Name: "use_standard_price", Type: core.ConnectionTypeBoolean},
	))
	if err != nil {
		t.Fatalf("a Salesforce refusal belongs on the error port: %v", err)
	}
	if ok, _ := out["success"].(bool); ok {
		t.Fatal("the price was not changed, so this cannot be a success")
	}
	msg, _ := out["error"].(string)
	if !strings.Contains(msg, "Copy The Standard Price") {
		t.Errorf("the message has to name the standard-price link, got: %q", msg)
	}
	// The shared translation still trails behind Salesforce's own wording (that
	// text belongs to common.go and every error carries it), but it must no longer
	// be the sentence the operator reads first — a price record has no address
	// fields, so State/Province can never be the cause here.
	standard := strings.Index(msg, "Copy The Standard Price")
	if address := strings.Index(msg, "State/Province"); address >= 0 && address < standard {
		t.Errorf("the address lecture is still leading the message: %q", msg)
	}
	// Salesforce's own wording is still carried, per the node's convention.
	if !strings.Contains(msg, "FIELD_INTEGRITY_EXCEPTION") {
		t.Errorf("the provider's own error should still be appended, got: %q", msg)
	}
}

// With the tick box actually sent, the wording that was already live-verified is
// unchanged — it can name Price Each as the thing to align.
func TestTickingTheBoxKeepsTheOriginalWording(t *testing.T) {
	defer refuseWithFieldIntegrity(t)()

	out, err := Execute(&core.Flow{}, &core.Node{}, priceInputs(
		&core.Connection{Name: "unit_price", Type: core.ConnectionTypeString, Value: "750"},
		&core.Connection{Name: "use_standard_price", Type: core.ConnectionTypeBoolean, Value: true},
	))
	if err != nil {
		t.Fatalf("a Salesforce refusal belongs on the error port: %v", err)
	}
	msg, _ := out["error"].(string)
	if !strings.Contains(msg, "Copy The Standard Price needs Price Each") {
		t.Errorf("unexpected wording for the ticked-box case: %q", msg)
	}
}
