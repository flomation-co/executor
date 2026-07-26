package crm_salesforce_opportunity_line_item_create

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// TestAnExplicitZeroQuantityIsRefusedNotRewrittenToOne is the regression test for
// the third copy of the same defect (Add Product to Quote and Add Product to Order
// carried it too). An explicit 0 was folded in with "not filled in", so a feed that
// maps a removed line to 0 had a line for ONE of the product written to the deal
// and reported as "Added 1 x product line". Salesforce refuses a zero quantity on
// a product line, so the honest outcome is a refusal.
//
// The check sits before any HTTP call when a price book entry is supplied
// directly, so this test needs no org: reaching Salesforce at all would be the
// failure.
func TestAnExplicitZeroQuantityIsRefusedNotRewrittenToOne(t *testing.T) {
	inputs := []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "opportunity_id", Type: core.ConnectionTypeString, Value: "006aj00000ZwpAzAAJ"},
		{Name: "pricebook_entry_id", Type: core.ConnectionTypeString, Value: "01uaj000008Qi5yAAC"},
		{Name: "unit_price", Type: core.ConnectionTypeString, Value: "25000"},
		{Name: "quantity", Type: core.ConnectionTypeString, Value: "0"},
	}

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs)
	if err == nil {
		t.Fatalf("a quantity of 0 must be refused, got: %v", out)
	}
	if !strings.Contains(err.Error(), "0") || !strings.Contains(err.Error(), "blank") {
		t.Errorf("the message should name the zero and the way out, got: %v", err)
	}
}
