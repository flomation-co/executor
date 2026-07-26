package crm_salesforce_product_upsert

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create or Update Product"
	Description  = "Keep your Salesforce catalogue in step with a spreadsheet, shop or ERP: match a product on its name, code or your own reference and update it if it exists, or create it if it does not. Safe to run over and over - it will not create duplicates."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+rotate"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// defaultMatchField is what an operator gets when they leave Match On blank.
//
// Name is chosen because it is the ONLY field on a stock Product2 that Salesforce
// will match an upsert against — verified live: Name is the sole createable field
// with idLookup=true, and PATCH /sobjects/Product2/Name/{value} answers 200 with
// created:false. See matchNotSupported below for what the alternatives do.
const defaultMatchField = "Name"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "match_field", Type: core.ConnectionTypeString, Label: "Match On", Placeholder: "Name (the default), or ProductCode, or StockKeepingUnit, or a custom External ID field such as ERP_Code__c"},
	{Name: "match_value", Type: core.ConnectionTypeString, Label: "Match Value", Placeholder: "GenWatt Diesel 200kW - the value to look for in that field", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Product Name", Placeholder: "GenWatt Diesel 200kW - only needed the first time, when the product has to be created"},
	{Name: "product_code", Type: core.ConnectionTypeString, Label: "Product Code", Placeholder: "GC1040 - your own catalogue code for this product"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "What the product is, in the words a customer would recognise"},
	{Name: "is_active", Type: core.ConnectionTypeBoolean, Label: "Ready To Sell (Salesforce creates new products switched OFF, so tick this on a first run)"},
	{Name: "family", Type: core.ConnectionTypeString, Label: "Product Family", Placeholder: "Must match a value in your org's Product Family list - Salesforce accepts anything else without complaining and stores it as typed"},
	{Name: "quantity_unit_of_measure", Type: core.ConnectionTypeString, Label: "Sold In Units Of", Placeholder: "Each - must match your org's Quantity Unit Of Measure list"},
	{Name: "stock_keeping_unit", Type: core.ConnectionTypeString, Label: "Stock Code (SKU)", Placeholder: "SKU-000142 - the code your warehouse or shop uses"},
	{Name: "display_url", Type: core.ConnectionTypeString, Label: "Product Web Page", Placeholder: "https://www.acme.co.uk/products/genwatt-200kw"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Your Reference", Placeholder: "ERP-000142 - a plain text box on the product; Salesforce does NOT treat it as an External ID"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the product"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Product"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	matchField := salesforce.OptionalString("match_field", inputs)
	if matchField == "" {
		matchField = defaultMatchField
	}
	// The match field is an identifier, not a value — it goes into the URL path
	// and into a SOQL WHERE clause, neither of which can quote it, so validating
	// it is the only defence available. It is also a configuration mistake that
	// never reaches Salesforce, so it takes the hard error return.
	if _, err := salesforce.ValidateSOQLFieldName(matchField); err != nil {
		return nil, fmt.Errorf("Match On — %w", err)
	}
	matchValue := salesforce.OptionalString("match_value", inputs)
	if matchValue == "" {
		return nil, fmt.Errorf("match_value is required — the value to look for in %s, e.g. the product code from your other system", matchField)
	}

	// Product Name is deliberately NOT required and is sent only when filled in.
	// Forcing it into every payload turns a field-level refresh — "keep the SKU
	// current, matched on ProductCode" — into a rename of every product on every
	// run, silently, with the run reporting success. Salesforce needs a Name only
	// on the half of the upsert that INSERTS and asks for it itself
	// (REQUIRED_FIELD_MISSING, translated on the way out) when the match finds
	// nothing. Same reasoning as account_upsert applies to Name.
	body := map[string]interface{}{}
	applyProductFields(body, inputs)
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	// The match field must not also appear in the body carrying a DIFFERENT value.
	// Both write paths strip it (the native upsert because Salesforce rejects the
	// field appearing in the URL and the body at once — "The Name field should not
	// be specified in the sobject data"), so the smuggled value is discarded and
	// the record keeps the value we matched on. The body can then be empty, which
	// still answers 200: a zero-write reported as "Updated".
	//
	// Checked against the MERGED body rather than the typed Product Name input, so
	// it also catches a rename posted through Additional Fields — the advanced
	// escape hatch is the path where an operator is most likely to try exactly
	// this, and it was the one route still silently dropping the value.
	if key, existing, ok := findFieldValue(body, matchField); ok {
		if text := strings.TrimSpace(fmt.Sprintf("%v", existing)); text != "" && !strings.EqualFold(text, matchValue) {
			hint := "match on something stable like Product Code instead, or change it with Update Product"
			if strings.EqualFold(matchField, "Name") {
				hint = "set Product Name to the same value, match on something stable like Product Code instead, or rename it with Update Product"
			}
			return nil, fmt.Errorf(
				"this is matching products on %s, so %s cannot also be used to change it — %q would be discarded and the product would keep %q. Either %s",
				matchField, key, text, matchValue, hint)
		}
		// Same value in both places is harmless; drop it so the write path does not
		// have to, and so an otherwise-empty body is visible as empty.
		delete(body, key)
	}

	// Native upsert first. UpsertRecord path-escapes the match value (a product
	// code containing "/" is ordinary) and strips the match field from the body,
	// which Salesforce rejects if it appears in both places.
	id, created, raw, err := salesforce.UpsertRecord(instanceURL, token, "Product2", matchField, matchValue, body)
	matchedManually := false
	if err != nil {
		switch {
		case multipleMatches(err):
			// Salesforce answers 300 Multiple Choices with a bare JSON array of
			// record URLs and NO errorCode at all (verified live on Name with two
			// products of the same name), so the operator would otherwise read
			// `Salesforce API error (300): ["/services/data/v62.0/..."]`.
			return salesforce.ErrorResult(fmt.Sprintf(
				"more than one product has %s = %q, so Salesforce cannot tell which one to update — make the value unique, or match on a field that only ever holds one product (%s)",
				matchField, matchValue, err.Error())), nil

		case matchNotSupported(err):
			// The honest, expensive discovery: a stock Product2 has NO External ID
			// field. ExternalId, ProductCode and StockKeepingUnit are all
			// externalId=false and idLookup=false on the live describe, and an
			// upsert against any of them fails with NOT_FOUND "Provided external
			// ID field does not exist or is not accessible: ProductCode" — every
			// one verified against the org.
			//
			// Product Code is nonetheless the key an operator syncing a
			// spreadsheet actually has, so refusing here would make the action
			// useless for its main job. Instead: find the product ourselves, then
			// update it or create it. Orgs that HAVE added a real External ID field
			// never reach this branch and keep Salesforce's own atomic upsert.
			id, created, raw, err = matchThenWrite(instanceURL, token, matchField, matchValue, body)
			if err != nil {
				return salesforce.ErrorResult(err.Error()), nil
			}
			matchedManually = true

		default:
			return salesforce.ErrorResult(err.Error()), nil
		}
	}

	record := raw
	if record == nil {
		record = map[string]interface{}{}
	}
	// An upsert that MATCHED could answer 204 No Content on older API versions, so
	// neither the ID nor the created flag would be in the body. Salesforce's own
	// 200/201 response carries a "created" key, so filling it in on the empty case
	// keeps the output shape identical either way — a downstream branch can test
	// it without caring which status came back.
	record["created"] = created
	if id == "" {
		// The 204 case carries no ID at all, and without it nothing downstream can
		// chain off the upsert — which is the whole point in a sync flow. Look it
		// up by the same match criteria. A failure here is not worth failing the
		// upsert over: the product was written, we just could not name it.
		//
		// BuildQueryTyped, not BuildQuery: a custom External ID field can be a
		// Number as easily as Text, and a numeric external ID quoted as '12345' is
		// a hard INVALID_FIELD — so the literal has to follow the FIELD's type,
		// which one cached describe settles.
		match := []salesforce.Condition{{Field: matchField, Operator: "=", Value: matchValue}}
		if soql, qerr := salesforce.BuildQueryTyped(instanceURL, token, "Product2", "Id,Name", match, false, "", 1, true); qerr == nil {
			if found, qerr := salesforce.QueryOne(instanceURL, token, soql); qerr == nil && found != nil {
				id = salesforce.StringifyID(found["Id"])
			}
		}
	}
	// Salesforce's own 201 body names the key "id", so fill that key rather than
	// inventing a second one — the raw result then looks the same to a downstream
	// node whether the product was inserted or matched.
	if _, present := record["id"]; !present && id != "" {
		record["id"] = id
	}
	// If even the lookup came back empty the write still succeeded, but the result
	// would be an anonymous {"created": false} with nothing in it to say WHICH
	// product was written. The match criteria are the only handle left, and they
	// are what the operator recognises anyway.
	if id == "" {
		record[matchField] = matchValue
	}

	verb := "Updated"
	if created {
		verb = "Created"
	}
	// Name the product by what Salesforce ACTUALLY stored, never by the box.
	//
	// Product2.Name is an idLookup field, so matching on Name — which is the
	// DEFAULT when Match On is left blank — takes the native upsert path, and
	// there UpsertRecord must strip the match field from the body because
	// Salesforce refuses it outright ("The Name field should not be specified in
	// the sobject data"). Reading the label from the input box therefore reported
	// a name that was never written. Verified live: matching on Name with a
	// different Product Name filled in created the product under the MATCH value
	// while the summary quoted the typed one, and a rename-only run wrote nothing
	// at all yet still reported "Updated".
	//
	// On the native Name path the stored name always equals the match value. On
	// the find-then-write fallback the body does carry Name, so the box is
	// truthful there. Prefer what came back, then the match value, and only fall
	// back to the box when the write genuinely carried it.
	label, _ := record["Name"].(string)
	if label == "" && strings.EqualFold(matchField, "Name") {
		label = matchValue
	}
	if label == "" {
		label = salesforce.OptionalString("name", inputs)
	}
	summary := fmt.Sprintf("%s product matched on %s = %q", verb, matchField, matchValue)
	if label != "" {
		summary = fmt.Sprintf("%s product %q matched on %s = %q", verb, label, matchField, matchValue)
	}
	// Both notes below can fire on the same run, so they are collected and joined
	// rather than appended with their own dashes — two "— ..." clauses in a row
	// read as a sentence that lost its way.
	var notes []string
	if created {
		// Same warning Create Product gives: a product with no price book entry
		// cannot go on a deal, and an operator whose sync "worked" needs to know
		// there is a second step.
		notes = append(notes, "a new product has no price yet, so add it to a price book before putting it on a deal")
	}
	if matchedManually {
		// Be explicit that this was a look-up-then-write rather than Salesforce's
		// own atomic upsert: two flows racing on the same value could each find
		// nothing and each create a product, which the native path cannot do.
		notes = append(notes, fmt.Sprintf("%s is not a Salesforce External ID field, so the product was found and then written in two steps — add a real External ID field to Product if two flows may sync the same product at once", matchField))
	}
	if len(notes) > 0 {
		summary += " — " + strings.Join(notes, "; ")
	}
	return salesforce.RecordResult(id, record, summary), nil
}

// matchThenWrite does by hand what Salesforce will not do on a field that is not
// an External ID: find the product, then update it or create it.
//
// It returns the same triple as UpsertRecord so the caller's result shaping is
// identical either way.
func matchThenWrite(instanceURL, token, matchField, matchValue string, body map[string]interface{}) (id string, created bool, raw map[string]interface{}, err error) {
	// LIMIT 2, not 1: one row means update, two mean the value is ambiguous and
	// the operator has to be told rather than have one of them picked at random.
	match := []salesforce.Condition{{Field: matchField, Operator: "=", Value: matchValue}}
	soql, err := salesforce.BuildQueryTyped(instanceURL, token, "Product2", "Id,Name", match, false, "", 2, true)
	if err != nil {
		return "", false, nil, err
	}
	records, _, _, _, err := salesforce.Query(instanceURL, token, soql, false, false)
	if err != nil {
		return "", false, nil, err
	}

	switch len(records) {
	case 0:
		// Creating: the match field has to carry its own value, or the product
		// would be created without the very reference the next run matches on.
		fields := make(map[string]interface{}, len(body)+1)
		for k, v := range body {
			fields[k] = v
		}
		fields[matchField] = matchValue
		newID, out, err := salesforce.CreateRecord(instanceURL, token, "Product2", fields)
		if err != nil {
			return "", false, nil, err
		}
		return newID, true, out, nil

	case 1:
		existingID := salesforce.StringifyID(records[0]["Id"])
		if len(body) == 0 {
			// Nothing to change is a legitimate outcome for a sync that only
			// wanted to know the product exists. UpdateRecord would refuse an
			// empty body, so answer directly rather than turning a no-op into a
			// failure.
			return existingID, false, records[0], nil
		}
		if err := salesforce.UpdateRecord(instanceURL, token, "Product2", existingID, body); err != nil {
			return "", false, nil, err
		}
		// An update answers 204 No Content, so echo what was applied — the same
		// shape Update Product returns.
		out := make(map[string]interface{}, len(body)+1)
		for k, v := range body {
			out[k] = v
		}
		out["Id"] = existingID
		if name, ok := records[0]["Name"].(string); ok && name != "" {
			if _, present := out["Name"]; !present {
				out["Name"] = name
			}
		}
		return existingID, false, out, nil

	default:
		return "", false, nil, fmt.Errorf(
			"more than one product has %s = %q, so there is no way to tell which one you meant — make the value unique in Salesforce, or match on a field that only ever holds one product",
			matchField, matchValue)
	}
}

// matchNotSupported reports whether Salesforce refused the upsert because the
// chosen field is not an External ID.
//
// It checks the code AND the message. NOT_FOUND is reused for a missing object
// and a missing endpoint, and treating either of those as "fall back to a manual
// match" would hide a genuine fault behind two extra writes.
func matchNotSupported(err error) bool {
	return salesforce.ErrorHasCode(err, "NOT_FOUND") &&
		strings.Contains(strings.ToLower(err.Error()), "external id field")
}

// multipleMatches reports whether Salesforce answered 300 Multiple Choices.
//
// There is nothing better to match on: that response is a bare JSON array of
// record URLs with no errorCode and no message, so CheckResponse has no envelope
// to translate and falls through to quoting the raw body. The status is the only
// signal the response carries.
func multipleMatches(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Salesforce API error (300)")
}

// applyProductFields maps the optional inputs onto their Salesforce API names.
// Unset inputs are omitted rather than sent blank, so an upsert that matches an
// existing product only touches the fields the flow actually filled in.
func applyProductFields(body map[string]interface{}, inputs []*core.Connection) {
	salesforce.SetIfPresent(body, inputs, "Name", "name")
	salesforce.SetIfPresent(body, inputs, "ProductCode", "product_code")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	// SetBoolIfSet, not a default: this action writes to products that already
	// exist, and forcing Ready To Sell on would un-retire a discontinued line
	// every time the catalogue synced. Create Product defaults it instead, where
	// there is no existing value to trample — and the label here warns that a
	// brand-new product lands switched off unless the box is ticked.
	salesforce.SetBoolIfSet(body, inputs, "IsActive", "is_active")
	salesforce.SetIfPresent(body, inputs, "Family", "family")
	salesforce.SetIfPresent(body, inputs, "QuantityUnitOfMeasure", "quantity_unit_of_measure")
	salesforce.SetIfPresent(body, inputs, "StockKeepingUnit", "stock_keeping_unit")
	salesforce.SetIfPresent(body, inputs, "DisplayUrl", "display_url")
	salesforce.SetIfPresent(body, inputs, "ExternalId", "external_id")
}

// findFieldValue looks a Salesforce field up in a body built partly from
// operator-supplied JSON, where capitalisation is whatever they typed. Returns
// the key AS WRITTEN so an error message quotes back what they actually put in.
func findFieldValue(body map[string]interface{}, field string) (string, interface{}, bool) {
	for k, v := range body {
		if strings.EqualFold(k, field) {
			return k, v, true
		}
	}
	return "", nil, false
}
