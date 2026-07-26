package crm_salesforce_price_book_entry_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Change Product Price"
	Description  = "Change what a product costs in a price book, or switch the price off so nobody can quote it. Anything you leave blank is left exactly as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "pricebook_entry_id", Type: core.ConnectionTypeString, Label: "Price Book Entry", Placeholder: "01u5f000000AbCdAAK - use Get Many Price Book Entries to find the price you want to change", Required: true},
	{Name: "unit_price", Type: core.ConnectionTypeString, Label: "Price Each", Placeholder: "27500.00 - plain numbers only, no currency symbols or commas"},
	{Name: "is_active", Type: core.ConnectionTypeBoolean, Label: "Price Can Be Used (untick to stop it being quoted)"},
	{Name: "use_standard_price", Type: core.ConnectionTypeBoolean, Label: "Copy The Standard Price (keeps this price in step with the list price; not for the standard price book itself)"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the price"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Price Book Entry ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Applied Changes"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	id := salesforce.OptionalString("pricebook_entry_id", inputs)
	if id == "" {
		return nil, fmt.Errorf("pricebook_entry_id is required — the price to change, e.g. 01u5f000000AbCdAAK. Use Get Many Price Book Entries to find it")
	}
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Every field is optional and only the ones that were filled in are sent: an
	// update that posted its blank inputs would set the price to nothing, which on
	// a live price book means a product that quotes at zero.
	body := map[string]interface{}{}
	unitPrice, unitSet, err := numericInput("unit_price", "Price Each", inputs)
	if err != nil {
		return nil, err
	}
	if unitSet {
		body["UnitPrice"] = unitPrice
	}
	// SetBoolIfSet, not a default: an untouched tick box means "leave the price as
	// it is". Create defaults these to usable because there is no existing value
	// to trample; here there is.
	salesforce.SetBoolIfSet(body, inputs, "IsActive", "is_active")
	salesforce.SetBoolIfSet(body, inputs, "UseStandardPrice", "use_standard_price")

	// Product and Price Book are deliberately NOT offered. Both are createable but
	// updateable=false on the live describe, and a PATCH carrying either answers
	// INVALID_FIELD_FOR_INSERT_UPDATE — verified. Moving a price to a different
	// product or book is not an edit, it is a delete and a re-add.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	// Read UseStandardPrice back off the FINISHED body rather than off the tick
	// box: Additional Fields can set it too, and gating the explanation below on
	// the tick box alone leaves the operator who typed {"UseStandardPrice":true}
	// reading common.go's translation, which is about address State/Province
	// picklists and has nothing to do with what happened.
	useStandard := standardPriceRequested(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in the price, or untick Price Can Be Used, to change something")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "PricebookEntry", id, body); err != nil {
		switch {
		case salesforce.ErrorHasCode(err, "INVALID_FIELD_FOR_INSERT_UPDATE"):
			// Only reachable through Additional Fields, since the two read-only
			// lookups are not offered as inputs — but that is exactly where an
			// operator copying a field name out of Salesforce will put them, and
			// the raw message talks about profile security settings, which sends
			// them to their administrator over something no permission can grant.
			return salesforce.ErrorResult(fmt.Sprintf(
				"one of the fields cannot be changed after the price is created — the Product and the Price Book are fixed for the life of a price, so delete this price and add a new one instead (%s)", err.Error())), nil

		case useStandard && salesforce.ErrorHasCode(err, "FIELD_INTEGRITY_EXCEPTION"):
			// Verified live: with the box ticked, Salesforce demands Price Each EQUAL
			// the product's standard price and refuses anything else with a bare
			// "field integrity exception". It also refuses the combination on the
			// standard book itself. common.go translates that code as an address
			// State/Province problem, which is not remotely what happened, so
			// intercept it whenever the request actually asked for the standard
			// price — that is what makes this the cause rather than a guess.
			return salesforce.ErrorResult(fmt.Sprintf(
				"Copy The Standard Price needs Price Each to be exactly the product's standard price — Salesforce does not fill it in for you. Either set Price Each to the standard price, or untick the box and set the price yourself. On the standard price book itself, untick it: the standard price IS the list price (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content — there is no updated
	// record to return. Echo back what was actually applied (plus the ID) so the
	// next node has something to work with and the execution view shows what
	// changed. Use Get Many Price Book Entries if the full record is needed.
	changed := salesforce.SortedKeys(body)
	record := make(map[string]interface{}, len(body)+1)
	for k, v := range body {
		record[k] = v
	}
	record["Id"] = id

	summary := fmt.Sprintf("Updated price %s — changed %s", id, strings.Join(changed, ", "))
	if unitSet {
		summary = fmt.Sprintf("Set price %s to %v — changed %s", id, unitPrice, strings.Join(changed, ", "))
	}
	return salesforce.RecordResult(id, record, summary), nil
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

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//
// OptionalFloat cannot tell "blank" from "£27,500" — both come back as unset, so
// a mistyped price would leave the old price in place while the run reported
// success, and the next quote would go out at last year's figure.
func numericInput(name, label string, inputs []*core.Connection) (float64, bool, error) {
	raw := salesforce.OptionalString(name, inputs)
	if raw == "" {
		return 0, false, nil
	}
	v, ok := salesforce.OptionalFloat(name, inputs)
	if !ok {
		return 0, false, fmt.Errorf("%s must be a plain number such as 27500.00 — got %q. Leave out currency symbols, thousands separators and spaces", label, raw)
	}
	return v, true, nil
}
