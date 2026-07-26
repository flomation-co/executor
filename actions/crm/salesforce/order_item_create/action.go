package crm_salesforce_order_item_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Product to Order"
	Description  = "Put a product line on an order. Pick the product and we find its price book entry, read the list price and put the order on the right price book for you - an order needs at least one product line before it can be activated."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+cart-shopping"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "order_id", Type: core.ConnectionTypeString, Label: "Order ID", Placeholder: "8015f000000AbCdAAK - the draft order to add the product to", Required: true},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - we find its price for you"},
	{Name: "pricebook_entry_id", Type: core.ConnectionTypeString, Label: "Price Book Entry", Placeholder: "01u5f000000AbCdAAK - only if you already know it; it wins over the Product above"},
	{Name: "quantity", Type: core.ConnectionTypeString, Label: "Quantity", Placeholder: "2 - how many of this product (defaults to 1)"},
	{Name: "unit_price", Type: core.ConnectionTypeString, Label: "Price Each", Placeholder: "499.00 - leave blank to use the price book price"},
	{Name: "service_date", Type: core.ConnectionTypeDateTime, Label: "Service Or Delivery Date", Placeholder: "2026-08-15 (the date only) - cannot be earlier than the order's own start date"},
	{Name: "end_date", Type: core.ConnectionTypeDateTime, Label: "Line End Date", Placeholder: "2027-07-31 (the date only) - for a subscription line; cannot be later than the order's own end date"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Line Description", Placeholder: "What this line covers, shown on the order"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the order line"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Order Line ID"},
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

	orderID := salesforce.OptionalString("order_id", inputs)
	if err := salesforce.ValidateRecordID(orderID); err != nil {
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

	// Salesforce does not let you attach a Product to an Order: you attach a
	// PricebookEntry, the product-plus-price-on-one-price-book join, and the entry
	// has to be on the price book the ORDER is on. Verified live, an order created
	// through the API has NO price book (Pricebook2Id comes back null even though
	// describe claims the field is defaulted on create), and the first line item
	// then fails with FIELD_INTEGRITY_EXCEPTION "The order is missing a price
	// book". Unlike an Opportunity, an Order is never pinned automatically — so
	// this action does it.
	order, err := readOrder(instanceURL, token, orderID)
	if err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"orders are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Order Settings ▸ Enable Orders (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}
	// Verified live: adding a product to an activated order is ENTITY_IS_LOCKED
	// ("cannot delete Order, or add or remove Order Products"). Checking it up
	// front means the operator is told what to do rather than shown a lock code,
	// and no pointless price-book lookup happens first.
	if order.status == "Activated" {
		// A PROVIDER outcome read back from Salesforce, not a configuration
		// mistake, so it belongs on the error port where a flow can branch on it —
		// the same treatment the ENTITY_IS_LOCKED catch further down already gets.
		return salesforce.ErrorResult(fmt.Sprintf("order %s is activated, so Salesforce has locked its product lines — set its Status back to Draft with Update Order, add the product, then activate it again", orderID)), nil
	}
	// A product line has to sit INSIDE its order's own dates. Both verified live:
	//
	//	line ServiceDate earlier than Order.EffectiveDate  FIELD_INTEGRITY_EXCEPTION
	//	line EndDate later than Order.EndDate              FIELD_INTEGRITY_EXCEPTION
	//	line EndDate on an order with no EndDate           OK — no ceiling to breach
	//
	// Checking here is worth doing even though Salesforce checks it too, because
	// the shared error translation for FIELD_INTEGRITY_EXCEPTION leads with the
	// address State/Country picklist — by far its commonest cause across the node,
	// and completely wrong here. An operator reading it goes hunting through
	// address fields for a date problem.
	if err := checkLineDates(order, salesforce.OptionalString("service_date", inputs), salesforce.OptionalString("end_date", inputs)); err != nil {
		return nil, err
	}
	orderBook := order.pricebookID

	// entryBookName is the price book's readable name, carried alongside its ID
	// so the summary can name the book rather than print an 18-character record ID at
	// an operator who cannot tell one from another.
	var entryBook, entryBookName string
	listPrice, havePrice := 0.0, false
	if entryID != "" {
		entryBook, entryBookName, listPrice, havePrice, err = readEntry(instanceURL, token, entryID)
	} else {
		entryID, entryBook, entryBookName, listPrice, havePrice, err = entryForProduct(instanceURL, token, productID, orderBook)
	}
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	pinned := false
	switch {
	case orderBook == "":
		if err := salesforce.UpdateRecord(instanceURL, token, "Order", orderID, map[string]interface{}{"Pricebook2Id": entryBook}); err != nil {
			return salesforce.ErrorResult(fmt.Sprintf("could not put order %s on price book %s, which is where this product is priced: %s", orderID, entryBook, err.Error())), nil
		}
		pinned = true
	case entryBook != "" && entryBook != orderBook:
		// A PROVIDER outcome, not a configuration mistake: this mismatch is only
		// discoverable by reading both records back from Salesforce, so a flow
		// should be able to branch on it — the same treatment the activated-order
		// check above and the quote twin's identical branch already get. A hard
		// error here took the whole node down and skipped the operator's own
		// error-handling step.
		return salesforce.ErrorResult(fmt.Sprintf("that price book entry is on price book %s but the order is on price book %s — Salesforce only allows lines priced from the order's own price book. Choose a product priced on %s, or change the order's price book first", entryBook, orderBook, orderBook)), nil
	}

	body := map[string]interface{}{
		"OrderId":          orderID,
		"PricebookEntryId": entryID,
	}

	// Quantity is mandatory on an order line and one is what an operator means
	// when they do not say — nobody orders "no" of something.
	//
	// An explicit 0 is a different instruction and is NOT folded in with "not
	// filled in". Salesforce refuses it outright ("Can't save order products with
	// quantities of zero", verified live), so rewriting it to 1 put a line the
	// source data never asked for on the order a warehouse picks from, behind a
	// green success. A shop or ERP feed that maps a removed line to 0 has to be
	// told, not quietly corrected.
	quantity, quantitySet, err := salesforce.NumericInput("quantity", "Quantity", "499.00", inputs)
	if err != nil {
		return nil, err
	}
	if quantitySet && quantity == 0 {
		return nil, fmt.Errorf("Quantity is 0, and Salesforce will not put a line for none of something on an order — leave Quantity blank to order one, or skip this product when the quantity coming in is zero")
	}
	if !quantitySet {
		quantity = 1
	}
	body["Quantity"] = quantity

	// UnitPrice is REQUIRED on OrderItem even though the describe reports it as
	// nillable — verified live: "Order Products must have a Unit Price." So the
	// fallback to the price book price is not a convenience, it is what stops the
	// action from demanding a figure the org already holds.
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
		return nil, fmt.Errorf("Salesforce will not add an order line without a price, and no list price could be read from the price book entry — fill in Price Each")
	}

	// There is deliberately no Line Total and no Discount input. OrderItem has no
	// Discount field at all, and TotalPrice is read-only — both verified against
	// the live org's describe. Salesforce computes the line total from quantity and
	// price; to discount a line, lower Price Each. ListPrice is left out too: it is
	// stamped automatically from the price book entry (verified live), and setting
	// it by hand only ever misreports what the product actually lists at.
	// ServiceDate and EndDate are Date fields — a full ISO timestamp is rejected.
	salesforce.SetDateIfPresent(body, inputs, "ServiceDate", "service_date")
	salesforce.SetDateIfPresent(body, inputs, "EndDate", "end_date")
	salesforce.SetIfPresent(body, inputs, "Description", "description")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "OrderItem", body)
	if err != nil {
		if salesforce.ErrorHasCode(err, "ENTITY_IS_LOCKED") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"that order is activated, so Salesforce has locked its product lines — set its Status back to Draft with Update Order, add the product, then activate it again (%s)", err.Error())), nil
		}
		// The shared translation for FIELD_INTEGRITY_EXCEPTION leads with the
		// address State/Country picklist, which is its commonest cause elsewhere in
		// this node and never its cause on an order line. Lead with what it
		// actually is here so the operator does not go looking at address fields.
		if salesforce.ErrorHasCode(err, "FIELD_INTEGRITY_EXCEPTION") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"Salesforce rejected this order line — on an order line this is normally the price book (the product must be priced on the order's own price book) or a date that falls outside the order's start and end dates. Salesforce's own wording is at the end: %s", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Added %v x product line to order %s (%s)", quantity, orderID, id)
	if pinned {
		label := entryBookName
		if label == "" {
			// A name is all but guaranteed (Pricebook2.Name is in the SELECT), but
			// never let a blank read produce "put on  to match the product".
			label = entryBook
		}
		summary += fmt.Sprintf(" — the order had no price book, so it was put on %s to match the product", label)
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

// orderHeader is the part of the order this action has to know about before it
// can add a line: which price book prices it, whether it is still editable, and
// the window its product lines have to fit inside.
type orderHeader struct {
	id            string
	pricebookID   string
	status        string
	effectiveDate string
	endDate       string
}

// readOrder reads the order header. An empty price book is normal on a freshly
// created order, not an error.
func readOrder(instanceURL, token, orderID string) (orderHeader, error) {
	soql, err := salesforce.BuildQuery(
		"Order",
		"Id,Pricebook2Id,Status,EffectiveDate,EndDate",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: orderID}},
		false, "", 1, true,
	)
	if err != nil {
		return orderHeader{}, err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return orderHeader{}, err
	}
	if record == nil {
		return orderHeader{}, fmt.Errorf("order %s was not found, or the connected Salesforce user cannot see it", orderID)
	}
	h := orderHeader{id: orderID, pricebookID: salesforce.StringifyID(record["Pricebook2Id"])}
	h.status, _ = record["Status"].(string)
	h.effectiveDate, _ = record["EffectiveDate"].(string)
	h.endDate, _ = record["EndDate"].(string)
	return h, nil
}

// checkLineDates enforces the order's own date window on the line being added.
func checkLineDates(order orderHeader, serviceDate, endDate string) error {
	start := dateOnly(serviceDate)
	if start != "" && order.effectiveDate != "" && start < order.effectiveDate {
		return fmt.Errorf("the service or delivery date (%s) is before order %s starts (%s) — Salesforce will not accept a product line that begins before its order does, so move the line's date, or move the order's start date back with Update Order", start, order.id, order.effectiveDate)
	}
	finish := dateOnly(endDate)
	if finish != "" && order.endDate != "" && finish > order.endDate {
		return fmt.Errorf("the line end date (%s) is after order %s ends (%s) — Salesforce will not accept a product line that runs past its order, so shorten the line, or extend the order's end date with Update Order", finish, order.id, order.endDate)
	}
	return nil
}

// dateOnly reduces an operator's date input to the YYYY-MM-DD form Salesforce
// stores, matching what SetDateIfPresent will actually send. Both sides of the
// comparison then use the same shape, so an upstream date picker's full ISO
// timestamp does not read as "later" than a plain date.
func dateOnly(v string) string {
	if len(v) > 10 && v[10] == 'T' {
		return v[:10]
	}
	return v
}

// readEntry reads a price book entry the operator supplied directly, returning
// which price book it belongs to and what it costs.
func readEntry(instanceURL, token, entryID string) (bookID, bookName string, price float64, havePrice bool, err error) {
	soql, err := salesforce.BuildQuery(
		"PricebookEntry",
		"Id,UnitPrice,Pricebook2Id,Product2Id",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: entryID}},
		false, "", 1, true,
	)
	if err != nil {
		return "", "", 0, false, err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return "", "", 0, false, err
	}
	if record == nil {
		return "", "", 0, false, fmt.Errorf("price book entry %s was not found, or the connected Salesforce user cannot see it", entryID)
	}
	bookID = salesforce.StringifyID(record["Pricebook2Id"])
	// Carry the book's readable name so a summary never has to print the ID.
	if pb, ok := record["Pricebook2"].(map[string]interface{}); ok {
		bookName, _ = pb["Name"].(string)
	}
	if p, ok := record["UnitPrice"].(float64); ok {
		price, havePrice = p, true
	}
	return bookID, bookName, price, havePrice, nil
}

// entryForProduct turns a product into the price book entry that prices it,
// reading the list price on the way past so the caller needs no second round trip.
//
// bookID narrows the search to one price book. When it is empty the search is
// widened to any ACTIVE entry on any ACTIVE price book — deliberately not "the
// standard price book", because a real org's standard price book is often left
// inactive while a working copy carries the prices (verified in the live test org,
// where IsStandard=true is inactive and the 17 usable entries sit on another book).
// Pinning an order to an inactive price book would just move the failure.
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
			describeCandidates(rows), "order")
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
			return "", "", "", 0, false, fmt.Errorf("that product is not priced on price book %s, which is the one this order uses — add it to that price book in Salesforce, or supply a price book entry directly", bookID)
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
