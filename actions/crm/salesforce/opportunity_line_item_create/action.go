package crm_salesforce_opportunity_line_item_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Product to Opportunity"
	Description  = "Put a product line on a deal. Pick the product and we work out its price book entry and list price for you - a deal with no product lines has no real value and never reaches the forecast."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+cart-shopping"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Opportunity ID", Placeholder: "0065f00000AbCdEAAV - the deal to add the product to", Required: true},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - we find its price for you"},
	{Name: "pricebook_entry_id", Type: core.ConnectionTypeString, Label: "Price Book Entry", Placeholder: "01u5f000000AbCdAAK - only if you already know it; otherwise leave blank"},
	{Name: "quantity", Type: core.ConnectionTypeString, Label: "Quantity", Placeholder: "1 - how many of this product (defaults to 1)"},
	{Name: "unit_price", Type: core.ConnectionTypeString, Label: "Price Each", Placeholder: "499.00 - leave blank to use the price book price"},
	{Name: "total_price", Type: core.ConnectionTypeString, Label: "Line Total", Placeholder: "1497.00 - set this instead of Price Each, never both"},
	{Name: "discount", Type: core.ConnectionTypeString, Label: "Discount (%)", Placeholder: "10 — only if Discount is enabled on Opportunity Products in your org"},
	{Name: "service_date", Type: core.ConnectionTypeDateTime, Label: "Service Or Delivery Date", Placeholder: "2026-10-01 (the date only)"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Line Description", Placeholder: "What this line covers, shown on the deal"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the product line"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product Line ID"},
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

	opportunityID := salesforce.OptionalString("opportunity_id", inputs)
	if err := salesforce.ValidateRecordID(opportunityID); err != nil {
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

	// Salesforce does not let you attach a Product to an Opportunity: you attach
	// a PricebookEntry, which is the product-plus-price-on-one-price-book join.
	// Working that ID out by hand is a three-step SOQL puzzle and is the single
	// thing that defeats non-technical users hand-rolling Salesforce calls, so
	// the action does it whenever only a product was chosen.
	listPrice, havePrice := 0.0, false
	if entryID == "" {
		entryID, listPrice, havePrice, err = resolveEntryForProduct(instanceURL, token, opportunityID, productID)
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
	}

	body := map[string]interface{}{
		"OpportunityId":    opportunityID,
		"PricebookEntryId": entryID,
	}

	// Quantity is mandatory on a product line and one is what an operator means
	// when they do not say — nobody adds "no" of something to a deal.
	//
	// An explicit 0 is a different instruction and is NOT folded in with "not
	// filled in": Salesforce refuses a zero quantity on a product line, so
	// rewriting it to 1 put a line the source data never asked for on the deal and
	// reported "Added 1 x product line" over the top of it.
	quantity, quantitySet, err := salesforce.NumericInput("quantity", "Quantity", "499.00", inputs)
	if err != nil {
		return nil, err
	}
	if quantitySet && quantity == 0 {
		return nil, fmt.Errorf("Quantity is 0, and Salesforce will not put a line for none of something on a deal — leave Quantity blank to add one, or skip this product when the quantity coming in is zero")
	}
	if !quantitySet {
		quantity = 1
	}
	body["Quantity"] = quantity

	// UnitPrice and TotalPrice are mutually exclusive in Salesforce — sending
	// both is rejected, and which one wins is not something to guess at.
	unitPrice, unitSet, err := salesforce.NumericInput("unit_price", "Price Each", "499.00", inputs)
	if err != nil {
		return nil, err
	}
	totalPrice, totalSet, err := salesforce.NumericInput("total_price", "Line Total", "499.00", inputs)
	if err != nil {
		return nil, err
	}
	switch {
	case unitSet && totalSet:
		return nil, fmt.Errorf("set either Price Each or Line Total, not both — Salesforce works the other one out from the quantity")
	case unitSet:
		body["UnitPrice"] = unitPrice
	case totalSet:
		body["TotalPrice"] = totalPrice
	default:
		// Neither was given, so fall back to what the price book says this
		// product costs. Salesforce requires a price on insert, and asking a
		// receptionist for one when the org already has a price list is exactly
		// the friction this node exists to remove.
		if !havePrice {
			listPrice, havePrice = lookupEntryPrice(instanceURL, token, entryID)
		}
		if havePrice {
			body["UnitPrice"] = listPrice
		}
	}

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

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "OpportunityLineItem", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	return salesforce.RecordResult(id, raw, fmt.Sprintf("Added %v x product line to opportunity %s (%s)", quantity, opportunityID, id)), nil
}

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//

// resolveEntryForProduct turns a product into the price book entry that prices
// it for this deal, and reads that entry's list price on the way past so the
// caller does not need a second round trip.
//
// The lookup walks the same path a Salesforce admin would: which price book is
// the deal on, and what does this product cost on it. A deal that has no price
// book yet (the normal state of a freshly created opportunity) falls back to the
// org's standard price book — adding the first line item is what makes
// Salesforce pin the deal to that price book, so this is not a guess.
func resolveEntryForProduct(instanceURL, token, opportunityID, productID string) (entryID string, listPrice float64, havePrice bool, err error) {
	pricebookID, err := opportunityPricebook(instanceURL, token, opportunityID)
	if err != nil {
		return "", 0, false, err
	}
	if pricebookID == "" {
		pricebookID, err = standardPricebook(instanceURL, token)
		if err != nil {
			return "", 0, false, err
		}
		if pricebookID == "" {
			return "", 0, false, fmt.Errorf("this deal has no price book and no standard price book was found in your Salesforce org — set the deal's price book first, or supply a price book entry directly")
		}
	}

	soql, err := salesforce.BuildQuery(
		"PricebookEntry",
		"Id,UnitPrice,Product2Id,Pricebook2Id",
		[]salesforce.Condition{
			{Field: "Product2Id", Operator: "=", Value: productID},
			{Field: "Pricebook2Id", Operator: "=", Value: pricebookID},
			{Field: "IsActive", Operator: "=", Value: "true"},
		},
		false, "", 1, true,
	)
	if err != nil {
		return "", 0, false, err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return "", 0, false, err
	}
	if record == nil {
		return "", 0, false, fmt.Errorf("that product is not on the price book this deal uses (%s) — add it to the price book in Salesforce, or supply a price book entry directly", pricebookID)
	}
	entryID = salesforce.StringifyID(record["Id"])
	if price, ok := record["UnitPrice"].(float64); ok {
		listPrice, havePrice = price, true
	}
	return entryID, listPrice, havePrice, nil
}

// opportunityPricebook reads the price book a deal is already pinned to.
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
		return "", fmt.Errorf("opportunity %s was not found, or the connected Salesforce user cannot see it", opportunityID)
	}
	return salesforce.StringifyID(record["Pricebook2Id"]), nil
}

// standardPricebook finds the org's standard price book, the fallback for a deal
// that has not been put on one yet.
func standardPricebook(instanceURL, token string) (string, error) {
	soql, err := salesforce.BuildQuery(
		"Pricebook2",
		"Id,Name",
		[]salesforce.Condition{
			{Field: "IsStandard", Operator: "=", Value: "true"},
			{Field: "IsActive", Operator: "=", Value: "true"},
		},
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
	return salesforce.StringifyID(record["Id"]), nil
}

// lookupEntryPrice reads a price book entry's list price, used when the operator
// supplied the entry directly and left the price blank. A failure here is not
// fatal: the field is simply omitted and Salesforce decides.
func lookupEntryPrice(instanceURL, token, entryID string) (float64, bool) {
	soql, err := salesforce.BuildQuery(
		"PricebookEntry",
		"Id,UnitPrice",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: entryID}},
		false, "", 1, true,
	)
	if err != nil {
		return 0, false
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil || record == nil {
		return 0, false
	}
	price, ok := record["UnitPrice"].(float64)
	return price, ok
}
