package crm_salesforce_product_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Product"
	Description  = "Change a product in your Salesforce catalogue - rename it, correct its code, or retire it so nobody can put it on a new deal. Anything you leave blank is left exactly as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - the record ID of the product to change", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Product Name", Placeholder: "GenWatt Diesel 200kW"},
	{Name: "product_code", Type: core.ConnectionTypeString, Label: "Product Code", Placeholder: "GC1040 - your own catalogue code for this product"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "What the product is, in the words a customer would recognise"},
	{Name: "is_active", Type: core.ConnectionTypeBoolean, Label: "Ready To Sell (untick to retire the product)"},
	{Name: "family", Type: core.ConnectionTypeString, Label: "Product Family", Placeholder: "Must match a value in your org's Product Family list - Salesforce accepts anything else without complaining and stores it as typed"},
	{Name: "quantity_unit_of_measure", Type: core.ConnectionTypeString, Label: "Sold In Units Of", Placeholder: "Each - must match your org's Quantity Unit Of Measure list"},
	{Name: "stock_keeping_unit", Type: core.ConnectionTypeString, Label: "Stock Code (SKU)", Placeholder: "SKU-000142 - the code your warehouse or shop uses"},
	{Name: "display_url", Type: core.ConnectionTypeString, Label: "Product Web Page", Placeholder: "https://www.acme.co.uk/products/genwatt-200kw"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Your Reference", Placeholder: "ERP-000142 - a plain text box on the product; Salesforce does NOT treat it as an External ID"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the product"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product ID"},
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

	id := salesforce.OptionalString("product_id", inputs)
	if id == "" {
		return nil, fmt.Errorf("product_id is required — the record ID of the product to change, e.g. 01t5f000004AbCdAAK")
	}
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Every field is optional and every one goes through Set*IfPresent: an update
	// that posted all its blank inputs would clear the operator's data, which on a
	// live catalogue means wiping the product code and description because they
	// only wanted to correct the name.
	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Name", "name")
	salesforce.SetIfPresent(body, inputs, "ProductCode", "product_code")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	// SetBoolIfSet, NOT the create action's default-to-active: an untouched tick
	// box here means "leave the product as it is". Defaulting it would silently
	// un-retire a product every time an unrelated field was corrected, which is
	// how a discontinued line reappears on next quarter's quotes.
	salesforce.SetBoolIfSet(body, inputs, "IsActive", "is_active")
	// Family and QuantityUnitOfMeasure are unrestricted picklists — Salesforce
	// accepts an unknown value silently (verified live), so the placeholders warn.
	salesforce.SetIfPresent(body, inputs, "Family", "family")
	salesforce.SetIfPresent(body, inputs, "QuantityUnitOfMeasure", "quantity_unit_of_measure")
	salesforce.SetIfPresent(body, inputs, "StockKeepingUnit", "stock_keeping_unit")
	salesforce.SetIfPresent(body, inputs, "DisplayUrl", "display_url")
	salesforce.SetIfPresent(body, inputs, "ExternalId", "external_id")

	// Bundle Type (Product2.Type) is deliberately absent: the live describe marks
	// it createable but updateable=false, so it can be set when the product is
	// created and never afterwards. Offering the box here would give the operator
	// a field that always fails.
	//
	// RecordTypeId is absent for the same class of reason — Product2 has no such
	// column in a stock org (verified live: INVALID_FIELD "No such column
	// 'RecordTypeId'"). Orgs that have switched product record types on can reach
	// it through Additional Fields.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change on the product")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Product2", id, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content — there is no updated
	// record to return. Echo back what was actually applied (plus the ID) so the
	// next node has something to work with and the execution view shows what
	// changed. Use Get Product if the full record is needed.
	changed := salesforce.SortedKeys(body)
	record := make(map[string]interface{}, len(body)+1)
	for k, v := range body {
		record[k] = v
	}
	record["Id"] = id

	return salesforce.RecordResult(id, record, fmt.Sprintf("Updated product %s — changed %s", id, strings.Join(changed, ", "))), nil
}
