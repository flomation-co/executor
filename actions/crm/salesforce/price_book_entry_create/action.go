package crm_salesforce_price_book_entry_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Product to Price Book"
	Description  = "Give a product a price in one of your price books, so it can be quoted and put on deals. Add it to the standard price book first - Salesforce insists on a list price before it will accept any other."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+tag"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "pricebook_id", Type: core.ConnectionTypeString, Label: "Price Book", Placeholder: "01s5f000004AbCdAAK - use Get Many Price Books to find it, and start with the standard one", Required: true},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - the product to give a price to", Required: true},
	{Name: "unit_price", Type: core.ConnectionTypeString, Label: "Price Each", Placeholder: "25000.00 - plain numbers only, no currency symbols or commas", Required: true},
	{Name: "is_active", Type: core.ConnectionTypeBoolean, Label: "Price Can Be Used (leave off and it is still switched on)"},
	{Name: "use_standard_price", Type: core.ConnectionTypeBoolean, Label: "Copy The Standard Price (keeps this price in step with the list price; not for the standard price book itself)"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the price"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Price Book Entry ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	pricebookID := salesforce.OptionalString("pricebook_id", inputs)
	if pricebookID == "" {
		return nil, fmt.Errorf("pricebook_id is required — the price book to put this price in, e.g. 01s5f000004AbCdAAK. Use Get Many Price Books to find it")
	}
	if err := salesforce.ValidateRecordID(pricebookID); err != nil {
		return nil, fmt.Errorf("Price Book — %w", err)
	}
	productID := salesforce.OptionalString("product_id", inputs)
	if productID == "" {
		return nil, fmt.Errorf("product_id is required — the product to give a price to, e.g. 01t5f000004AbCdAAK")
	}
	if err := salesforce.ValidateRecordID(productID); err != nil {
		return nil, fmt.Errorf("Product — %w", err)
	}

	// UnitPrice is required on EVERY price book entry — including one that says
	// Copy The Standard Price. Verified live: UseStandardPrice=true with no
	// UnitPrice answers REQUIRED_FIELD_MISSING [UnitPrice], which is surprising
	// enough that leaving the operator to discover it is unkind.
	//
	// And it must EQUAL the product's standard price, not merely be present.
	// Salesforce does NOT overwrite the figure — verified live against a product
	// with a standard price of 500: UnitPrice 1 with the box ticked is refused
	// with FIELD_INTEGRITY_EXCEPTION, UnitPrice 500 is accepted. An earlier
	// version of this comment claimed the opposite and told the operator any
	// sensible number would do, which is the worst kind of wrong: it reads as
	// reassurance and produces a failure.
	unitPrice, unitSet, err := salesforce.NumericInput("unit_price", "Price Each", "25000.00", inputs)
	if err != nil {
		return nil, err
	}
	if !unitSet {
		return nil, fmt.Errorf("the price is required — what one of this product costs on this price book, e.g. 25000.00. Salesforce will not accept a price book entry without one, even when Copy The Standard Price is ticked")
	}

	body := map[string]interface{}{
		"Pricebook2Id": pricebookID,
		"Product2Id":   productID,
		"UnitPrice":    unitPrice,
	}

	// Salesforce defaults PricebookEntry.IsActive to FALSE (live describe,
	// defaultValue=false). An inactive entry cannot be put on a deal and does not
	// appear in the product picker, so a price created through the API and left
	// alone is invisible — with a 201 and no hint that anything is wrong. An
	// untouched tick box therefore means usable, and the label says so. This
	// matches Create Product, which overrides the same Salesforce default for the
	// same reason.
	if v, set := boolInput("is_active", inputs); set {
		body["IsActive"] = v
	} else {
		body["IsActive"] = true
	}

	salesforce.SetBoolIfSet(body, inputs, "UseStandardPrice", "use_standard_price")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	// Read UseStandardPrice back off the FINISHED body rather than off the tick
	// box, because Additional Fields can set it too — and if the explanation
	// below is gated on the tick box alone, the operator who typed
	// {"UseStandardPrice":true} gets common.go's translation instead, which is
	// about address State/Province picklists and sends them somewhere useless.
	useStandard := standardPriceRequested(body)

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "PricebookEntry", body)
	if err != nil {
		// Three provider outcomes are common enough — and badly enough explained
		// by Salesforce or by common.go — to name explicitly. All three were
		// reproduced against a live org.
		switch {
		case salesforce.ErrorHasCode(err, "STANDARD_PRICE_NOT_DEFINED"):
			// Salesforce's own text is "Before creating a custom price, create a
			// standard price." — true, and completely opaque to someone who has
			// never heard of a standard price book. This is THE wall every
			// catalogue import hits, so it gets the full explanation plus the ID of
			// the book they actually need.
			return salesforce.ErrorResult(fmt.Sprintf(
				"Salesforce will not price a product on this price book until it has a list price on the STANDARD price book%s — run this action again against the standard price book first, then come back to this one (%s)",
				standardPricebookHint(instanceURL, token), err.Error())), nil

		// Both of the next two cases arrive as FIELD_INTEGRITY_EXCEPTION, so they
		// can only be told apart by the MESSAGE. Ordering matters: the duplicate
		// carries specific text, the standard-price mismatch is bare.
		//
		// This branch previously tested for DUPLICATE_VALUE, which Salesforce
		// never sends here — verified live, a second entry for the same product
		// and book answers FIELD_INTEGRITY_EXCEPTION with "This price definition
		// already exists in this price book". So the branch was DEAD CODE, the
		// advice never reached anyone, and re-running a catalogue sync instead
		// fell through to common.go's translation of that code, which is about
		// address State/Province picklists and has nothing to do with pricing.
		case strings.Contains(err.Error(), "already exists in this price book"):
			// A product can hold only ONE price per price book, so the operator
			// wants Change Product Price, not a second entry. This is the ordinary
			// outcome of re-running a catalogue sync.
			return salesforce.ErrorResult(fmt.Sprintf(
				"that product already has a price in that price book — a product can only have one price per price book, so use Change Product Price to change it, or Get Many Price Book Entries to find the existing one (%s)", err.Error())), nil

		case useStandard && salesforce.ErrorHasCode(err, "FIELD_INTEGRITY_EXCEPTION"):
			// Verified live: with the box ticked, Salesforce demands Price Each
			// EQUAL the product's standard price and refuses anything else with a
			// bare "field integrity exception" carrying no detail. It also refuses
			// the combination on the standard book itself. Either way the operator
			// needs to know the figure has to match, which the previous wording did
			// not say — it named the standard-book case only, so an operator using
			// an ordinary price book was told the one thing they had got right.
			return salesforce.ErrorResult(fmt.Sprintf(
				"Copy The Standard Price needs Price Each to be exactly the product's standard price — Salesforce does not fill it in for you. Either set Price Each to the standard price, or untick the box and price it yourself. On the standard price book itself, untick it: the standard price IS the list price (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Name the product and the book rather than printing two 18-character IDs at
	// whoever reads the run. The product in particular is usually NOT something
	// the operator picked here: the canonical flow is Create or Update Product
	// followed by this action, with Product bound to the upstream node's id, so
	// nobody has ever seen the product's name on this node.
	product, book := entryLabels(instanceURL, token, id)
	if product == "" {
		// Falls back to the exact previous wording, so a failed lookup reads as it
		// always did rather than as a half-finished sentence.
		product = "product " + productID
	} else {
		product = fmt.Sprintf("%q", product)
	}
	if book == "" {
		book = pricebookID
	}

	summary := fmt.Sprintf("Priced %s at %v on price book %s (%s)", product, unitPrice, book, id)
	if useStandard {
		// Salesforce overwrites the figure that was sent with the list price when
		// this is on, so quoting the operator's number back at them would name a
		// price the record does not hold.
		summary = fmt.Sprintf("Priced %s on price book %s using the standard list price (%s)", product, book, id)
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

// entryLabels reads back the names behind the two IDs in one query, for the
// summary only.
//
// Deliberately fail-open: any failure returns blanks and the caller falls back to
// the IDs. This is a COSMETIC read on the success path, so it must never turn a
// completed write into an error — the price exists by the time this runs, and
// reporting failure would send the operator to create it a second time.
//
// (The opposite rule applies to a safety read: syncLineCounts in
// quote_sync_to_opportunity refuses the sync when it cannot count, because
// proceeding risks deleting a deal's product lines. Safety reads fail closed,
// cosmetic reads fail open.)
func entryLabels(instanceURL, token, entryID string) (product, book string) {
	soql, err := salesforce.BuildQuery(
		"PricebookEntry",
		"Id,Product2.Name,Pricebook2.Name",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: entryID}},
		false, "", 1, true,
	)
	if err != nil {
		return "", ""
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil || record == nil {
		return "", ""
	}
	if p, ok := record["Product2"].(map[string]interface{}); ok {
		product, _ = p["Name"].(string)
	}
	if b, ok := record["Pricebook2"].(map[string]interface{}); ok {
		book, _ = b["Name"].(string)
	}
	return product, book
}

// standardPriceRequested reports whether the request actually asked Salesforce to
// copy the standard price, from wherever the value came.
//
// The tick box writes a real bool and Additional Fields writes whatever the
// operator's JSON decoded to, so both forms are accepted — a string "true" typed
// into Additional Fields is the same instruction as a ticked box.
func standardPriceRequested(body map[string]interface{}) bool {
	switch v := body["UseStandardPrice"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return false
}

// standardPricebookHint names the org's standard price book so the
// STANDARD_PRICE_NOT_DEFINED message points at a record rather than a concept.
//
// It deliberately does NOT filter on IsActive. Verified live: a stock Developer
// Edition org's standard price book has IsStandard=true and IsActive=FALSE, so an
// active-only lookup finds nothing and the hint silently disappears exactly when
// it is needed. Returns "" on any failure — this runs on the error path and must
// never turn one failure into two.
func standardPricebookHint(instanceURL, token string) string {
	soql, err := salesforce.BuildQuery(
		"Pricebook2",
		"Id,Name",
		[]salesforce.Condition{{Field: "IsStandard", Operator: "=", Value: "true"}},
		false, "", 1, true,
	)
	if err != nil {
		return ""
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil || record == nil {
		return ""
	}
	id := salesforce.StringifyID(record["Id"])
	name, _ := record["Name"].(string)
	switch {
	case name != "" && id != "":
		return fmt.Sprintf(" (yours is %q, %s)", name, id)
	case id != "":
		return fmt.Sprintf(" (yours is %s)", id)
	}
	return ""
}

// boolInput reads a tick box, reporting separately whether the operator touched
// it. OptionalBool collapses "unticked" and "never touched" into false, which is
// exactly the distinction this action needs in order to default to usable.
func boolInput(name string, inputs []*core.Connection) (value, set bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false, false
	}
	return *conn.Boolean(), true
}

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//
