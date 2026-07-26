package crm_salesforce_quote_line_item_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Product to Quote"
	Description  = "Put a product line on a quote. Pick the product and we find its price book entry, read the list price and put the quote on the right price book for you - a quote with no product lines totals nothing and cannot be sent."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+cart-shopping"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "quote_id", Type: core.ConnectionTypeString, Label: "Quote ID", Placeholder: "0Q05f000000AbCdAAK - the quote to add the product to", Required: true},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - we find its price for you"},
	{Name: "pricebook_entry_id", Type: core.ConnectionTypeString, Label: "Price Book Entry", Placeholder: "01u5f000000AbCdAAK - only if you already know it; it wins over the Product above"},
	{Name: "quantity", Type: core.ConnectionTypeString, Label: "Quantity", Placeholder: "1 - how many of this product (defaults to 1)"},
	{Name: "unit_price", Type: core.ConnectionTypeString, Label: "Price Each", Placeholder: "499.00 - leave blank to use the price book price"},
	{Name: "discount", Type: core.ConnectionTypeString, Label: "Discount (%)", Placeholder: "10 - a percentage off this line, which is what drives the quote's overall discount"},
	{Name: "service_date", Type: core.ConnectionTypeDateTime, Label: "Service Or Delivery Date", Placeholder: "2026-10-01 (the date only)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Line Description", Placeholder: "What this line covers, printed on the quote"},
	{Name: "sort_order", Type: core.ConnectionTypeInteger, Label: "Line Order", Placeholder: "1 - where this line appears on the printed quote"},
	{Name: "opportunity_line_item_id", Type: core.ConnectionTypeString, Label: "Deal Product Line", Placeholder: "00k5f000000AbCdAAK - link this line back to the matching product line on the deal"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the quote line"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Quote Line ID"},
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

	quoteID := salesforce.OptionalString("quote_id", inputs)
	if err := salesforce.ValidateRecordID(quoteID); err != nil {
		return nil, err
	}

	entryID := salesforce.OptionalString("pricebook_entry_id", inputs)
	productID := salesforce.OptionalString("product_id", inputs)
	if entryID == "" && productID == "" {
		return nil, fmt.Errorf("choose a product to add — or, if you already know it, supply the price book entry instead")
	}
	if entryID != "" {
		if err := salesforce.ValidateRecordID(entryID); err != nil {
			return nil, err
		}
	}
	if productID != "" {
		if err := salesforce.ValidateRecordID(productID); err != nil {
			return nil, err
		}
	}

	// Salesforce does not let you attach a Product to a Quote: you attach a
	// PricebookEntry, the product-plus-price-on-one-price-book join. And the entry
	// has to be on the price book the QUOTE is on — verified live, a mismatch (or
	// a quote with no price book at all) is FIELD_INTEGRITY_EXCEPTION "The price
	// book entry is in a different price book than the one assigned to the Quote".
	// Working all of that out by hand is a four-step SOQL puzzle, so the action
	// does it.
	quoteBook, opportunityID, err := readQuote(instanceURL, token, quoteID)
	if err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"quotes are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Quotes ▸ Quote Settings (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// entryBookName is the price book's readable name, carried alongside its ID
	// so the summary can name the book rather than print an 18-character record ID at
	// an operator who cannot tell one from another.
	var entryBook, entryBookName string
	listPrice, havePrice := 0.0, false
	// entryForProduct only ever returns an ACTIVE entry (it filters on IsActive),
	// so this stays true on that path and is only really read for an entry the
	// operator supplied themselves.
	entryActive := true
	if entryID != "" {
		entryBook, entryBookName, listPrice, havePrice, entryActive, err = readEntry(instanceURL, token, entryID)
	} else {
		// Prefer the quote's own price book; failing that the parent deal's, so a
		// quote generated from a deal is priced the same way the deal is.
		target := quoteBook
		if target == "" && opportunityID != "" {
			target, err = opportunityPricebook(instanceURL, token, opportunityID)
			if err != nil {
				return salesforce.ErrorResult(err.Error()), nil
			}
		}
		entryID, entryBook, entryBookName, listPrice, havePrice, err = entryForProduct(instanceURL, token, productID, target)
	}
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// A switched-off price cannot go on a quote: Salesforce answers
	// FIELD_INTEGRITY_EXCEPTION "The price book entry is inactive. Ask your
	// Salesforce admin for help." (verified live) — and admin help is not what is
	// needed, because this node fixes it in one step. Salesforce DEFAULTS
	// PricebookEntry.IsActive to false, and Get Many Price Book Entries has "Only
	// Prices That Can Be Used" off by default, so an inactive entry ID arriving
	// here from a list is ordinary rather than exotic. Refusing before the write
	// also keeps the quote from being pinned to a price book for a price it can
	// never use.
	if !entryActive {
		return salesforce.ErrorResult(fmt.Sprintf("price %s is switched off in Salesforce, so it cannot go on a quote — tick Price Can Be Used for it with Change Product Price, or choose another price. Salesforce switches new prices off by default, so a catalogue loaded through an API often has them", entryID)), nil
	}

	// Pin the quote to the price book the line is priced from. Salesforce will
	// not do this itself (unlike an Opportunity, which is pinned by its first
	// line item), so without it the very next call fails — and the field was
	// empty, so filling it cannot overwrite anything the operator chose.
	pinned := false
	switch {
	case quoteBook == "":
		if err := salesforce.UpdateRecord(instanceURL, token, "Quote", quoteID, map[string]interface{}{"Pricebook2Id": entryBook}); err != nil {
			return salesforce.ErrorResult(fmt.Sprintf("could not put quote %s on price book %s, which is where this product is priced: %s", quoteID, entryBook, err.Error())), nil
		}
		pinned = true
	case entryBook != "" && entryBook != quoteBook:
		// A PROVIDER outcome, not a configuration mistake: this mismatch is only
		// discoverable by reading both records back from Salesforce, so a flow
		// should be able to branch on it. Returning a hard error took the node
		// down instead of reaching the error port, unlike the ENTITY_IS_LOCKED
		// case below which was already soft.
		return salesforce.ErrorResult(fmt.Sprintf("that price book entry is on price book %s but the quote is on price book %s — Salesforce only allows lines priced from the quote's own price book. Choose a product priced on %s, or change the quote's price book first", entryBook, quoteBook, quoteBook)), nil
	}

	body := map[string]interface{}{
		"QuoteId":          quoteID,
		"PricebookEntryId": entryID,
	}

	// Quantity is mandatory on a quote line and one is what an operator means
	// when they do not say — nobody quotes "no" of something.
	//
	// An explicit 0 is a different instruction and is NOT folded in with "not
	// filled in". Salesforce refuses a zero quantity outright ("Quantity must be
	// nonzero", verified live), so rewriting it to 1 put a line the source data
	// never asked for on a customer-facing quote and then reported "Added 1 x
	// product line" over the top of it. A shop or ERP feed that maps a removed
	// line to 0 has to be told, not quietly corrected.
	quantity, quantitySet, err := salesforce.NumericInput("quantity", "Quantity", "499.00", inputs)
	if err != nil {
		return nil, err
	}
	if quantitySet && quantity == 0 {
		return nil, fmt.Errorf("Quantity is 0, and Salesforce will not put a line for none of something on a quote — leave Quantity blank to quote one, or skip this product when the quantity coming in is zero")
	}
	if !quantitySet {
		quantity = 1
	}
	body["Quantity"] = quantity

	// UnitPrice is REQUIRED on QuoteLineItem even though the describe reports it
	// as nillable — verified live: an insert without it is REQUIRED_FIELD_MISSING
	// [UnitPrice]. (OpportunityLineItem is genuinely optional here, so the two
	// line-item actions cannot share this logic.) Falling back to the price book
	// price is therefore not a convenience, it is what makes the action usable
	// without asking a receptionist for a figure the org already holds.
	unitPrice, unitSet, err := salesforce.NumericInput("unit_price", "Price Each", "499.00", inputs)
	if err != nil {
		return nil, err
	}
	switch {
	case unitSet:
		body["UnitPrice"] = unitPrice
	case havePrice:
		body["UnitPrice"] = listPrice
	default:
		return nil, fmt.Errorf("Salesforce will not add a quote line without a price, and no list price could be read from the price book entry — fill in Price Each")
	}

	// There is deliberately no Line Total input. QuoteLineItem.TotalPrice is
	// read-only — verified live, sending it is INVALID_FIELD_FOR_INSERT_UPDATE —
	// because Salesforce computes it from quantity, price and discount. This is
	// the one place quote lines differ from opportunity lines, where TotalPrice
	// IS writable.
	discount, discountSet, err := salesforce.NumericInput("discount", "Discount", "499.00", inputs)
	if err != nil {
		return nil, err
	}
	if discountSet {
		body["Discount"] = discount
	}
	// ServiceDate is a Date field — a full ISO timestamp is rejected outright.
	salesforce.SetDateIfPresent(body, inputs, "ServiceDate", "service_date")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIntIfPresent(body, inputs, "SortOrder", "sort_order")
	salesforce.SetIfPresent(body, inputs, "OpportunityLineItemId", "opportunity_line_item_id")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "QuoteLineItem", body)
	if err != nil {
		// The shared translation for FIELD_INTEGRITY_EXCEPTION leads with the
		// address State/Country picklist — its commonest cause across this node, and
		// never its cause on a quote line, where it is the price book. Lead with
		// what it actually is so the operator does not go hunting through address
		// fields. (A quote line, unlike an order line, has no date window to
		// breach — verified live, a service date years past the quote's expiry is
		// accepted.)
		if salesforce.ErrorHasCode(err, "FIELD_INTEGRITY_EXCEPTION") {
			// Test the inactive case FIRST. The entry was active when it was read a
			// moment ago, but it can be switched off in between, and Additional
			// Fields can carry an entry ID this action never read at all. Leading
			// with the price book there would be a second wrong diagnosis on top of
			// the address one this branch exists to suppress.
			if strings.Contains(strings.ToLower(err.Error()), "inactive") {
				return salesforce.ErrorResult(fmt.Sprintf(
					"that price is switched off in Salesforce, so it cannot go on a quote — tick Price Can Be Used for it with Change Product Price, or choose another price (%s)", err.Error())), nil
			}
			return salesforce.ErrorResult(fmt.Sprintf(
				"Salesforce rejected this quote line — on a quote line this is almost always the price book: the product has to be priced on the quote's own price book. Salesforce's own wording is at the end: %s", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Added %v x product line to quote %s (%s)", quantity, quoteID, id)
	if pinned {
		label := entryBookName
		if label == "" {
			// A name is all but guaranteed (Pricebook2.Name is in the SELECT), but
			// never let a blank read produce "put on  to match the product".
			label = entryBook
		}
		summary += fmt.Sprintf(" — the quote had no price book, so it was put on %s to match the product", label)
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

// readQuote reads the price book a quote is on and the deal behind it. Both may
// legitimately be empty on a freshly created quote, which is not an error.
func readQuote(instanceURL, token, quoteID string) (pricebookID, opportunityID string, err error) {
	soql, err := salesforce.BuildQuery(
		"Quote",
		"Id,Pricebook2Id,OpportunityId",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: quoteID}},
		false, "", 1, true,
	)
	if err != nil {
		return "", "", err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return "", "", err
	}
	if record == nil {
		return "", "", fmt.Errorf("quote %s was not found, or the connected Salesforce user cannot see it", quoteID)
	}
	return salesforce.StringifyID(record["Pricebook2Id"]), salesforce.StringifyID(record["OpportunityId"]), nil
}

// opportunityPricebook reads the price book the parent deal is pinned to.
// Returns "" when the deal has none yet, which is not an error.
func opportunityPricebook(instanceURL, token, opportunityID string) (string, error) {
	soql, err := salesforce.BuildQuery(
		"Opportunity",
		"Id,Pricebook2Id",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: opportunityID}},
		false, "", 1, true,
	)
	if err != nil {
		return "", err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", nil
	}
	return salesforce.StringifyID(record["Pricebook2Id"]), nil
}

// readEntry reads a price book entry the operator supplied directly, returning
// which price book it belongs to, what it costs, and whether it can actually be
// used — IsActive is read here so the caller can say "that price is switched
// off" instead of letting Salesforce answer with a field-integrity code.
func readEntry(instanceURL, token, entryID string) (bookID, bookName string, price float64, havePrice, active bool, err error) {
	soql, err := salesforce.BuildQuery(
		"PricebookEntry",
		"Id,UnitPrice,Pricebook2Id,Pricebook2.Name,Product2Id,IsActive",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: entryID}},
		false, "", 1, true,
	)
	if err != nil {
		return "", "", 0, false, false, err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return "", "", 0, false, false, err
	}
	if record == nil {
		return "", "", 0, false, false, fmt.Errorf("price book entry %s was not found, or the connected Salesforce user cannot see it", entryID)
	}
	bookID = salesforce.StringifyID(record["Pricebook2Id"])
	// Carry the book's readable name so a summary never has to print the ID.
	if pb, ok := record["Pricebook2"].(map[string]interface{}); ok {
		bookName, _ = pb["Name"].(string)
	}
	if p, ok := record["UnitPrice"].(float64); ok {
		price, havePrice = p, true
	}
	// A missing IsActive (an org that denies the field, or a stubbed response)
	// must not read as "switched off" and block a write Salesforce would accept.
	active = true
	if a, ok := record["IsActive"].(bool); ok {
		active = a
	}
	return bookID, bookName, price, havePrice, active, nil
}

// entryForProduct turns a product into the price book entry that prices it,
// reading the list price on the way past so the caller needs no second round
// trip.
//
// bookID narrows the search to one price book. When it is empty the search is
// widened to any ACTIVE entry on any ACTIVE price book — deliberately not "the
// standard price book", because a real org's standard price book is often left
// inactive while a working copy carries the prices (verified in the live test org,
// where IsStandard=true is inactive and the 17 usable entries are on another
// book). Pinning a quote to an inactive price book would just move the failure.
func entryForProduct(instanceURL, token, productID, bookID string) (entryID, entryBook, bookName string, price float64, havePrice bool, err error) {
	conditions := []salesforce.Condition{
		{Field: "Product2Id", Operator: "=", Value: productID},
		{Field: "IsActive", Operator: "=", Value: "true"},
	}
	if bookID != "" {
		conditions = append(conditions, salesforce.Condition{Field: "Pricebook2Id", Operator: "=", Value: bookID})
	} else {
		conditions = append(conditions, salesforce.Condition{Field: "Pricebook2.IsActive", Operator: "=", Value: "true"})
	}
	// LIMIT 2, not 1, and select the book's NAME. With no book to narrow by, one
	// row means the choice is unambiguous and two mean it is not — and a product
	// priced on several active books is the ordinary Salesforce setup
	// (Retail / Wholesale / Partner), not an edge case.
	//
	// This used to take LIMIT 1 with no ORDER BY, so Salesforce chose the row.
	// Reviewed live: a product priced £25,000 on Standard and £1.00 on Wholesale
	// returned the £1.00 row, the line was written at £1.00, and the document was
	// then PINNED to the wholesale book so every later line followed it. Picking
	// a price at random is the one outcome a CRM must never produce quietly, so
	// this now refuses and names the candidates. Same reasoning as the LIMIT 2
	// ambiguity check in product_upsert.
	soql, err := salesforce.BuildQuery(
		"PricebookEntry",
		"Id,UnitPrice,Product2Id,Pricebook2Id,Pricebook2.Name",
		conditions,
		false, "", 2, true,
	)
	if err != nil {
		return "", "", "", 0, false, err
	}
	rows, _, _, _, err := salesforce.Query(instanceURL, token, soql, false, false)
	if err != nil {
		return "", "", "", 0, false, err
	}
	if bookID == "" && len(rows) > 1 {
		return "", "", "", 0, false, fmt.Errorf(
			"that product has a price on more than one active price book (%s), so there is no single price to use — set the price book on the %s first, or choose the price book entry instead of the product",
			describeCandidates(rows), "quote")
	}
	var record map[string]interface{}
	if len(rows) > 0 {
		record = rows[0]
		// Carry the book's NAME out with the entry. The summary used to print the
		// 18-character Pricebook2Id at a non-technical operator, who has no way to
		// tell which book that is — and this action can PIN the document to it, so
		// the one line telling them what happened has to be readable.
		if pb, ok := record["Pricebook2"].(map[string]interface{}); ok {
			bookName, _ = pb["Name"].(string)
		}
	}
	if record == nil {
		if bookID != "" {
			// Deliberately not "the price book this quote uses": bookID is the
			// quote's own book when it has one and the PARENT DEAL's when it does
			// not (the quote is then pinned to it below), and telling an operator
			// their bookless quote uses a price book sends them looking at a field
			// that is empty.
			return "", "", "", 0, false, fmt.Errorf("that product is not priced on price book %s, which is the price book this quote is priced from — add it to that price book in Salesforce, or supply a price book entry directly", bookID)
		}
		return "", "", "", 0, false, fmt.Errorf("that product has no active price on any active price book in your Salesforce org — give it a price under the product's Price Books related list, or supply a price book entry directly")
	}
	entryID = salesforce.StringifyID(record["Id"])
	entryBook = salesforce.StringifyID(record["Pricebook2Id"])
	if p, ok := record["UnitPrice"].(float64); ok {
		price, havePrice = p, true
	}
	return entryID, entryBook, bookName, price, havePrice, nil
}

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//

// describeCandidates names the competing price books and their prices, so the
// operator can see WHY the choice was refused and what the options were. A raw
// list of entry IDs would leave them no better off than the silent guess did.
func describeCandidates(rows []map[string]interface{}) string {
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		name := "an unnamed price book"
		if pb, ok := r["Pricebook2"].(map[string]interface{}); ok {
			if n, ok := pb["Name"].(string); ok && n != "" {
				name = n
			}
		}
		if p, ok := r["UnitPrice"].(float64); ok {
			parts = append(parts, fmt.Sprintf("%s at %.2f", name, p))
			continue
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, ", ")
}
