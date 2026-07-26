package crm_salesforce_product_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Product"
	Description  = "Add a product to your Salesforce catalogue so it can be priced, quoted and put on deals. New products are created ready to sell unless you turn that off."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+plus"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Product Name", Placeholder: "GenWatt Diesel 200kW - what the product is called on quotes and deals", Required: true},
	{Name: "product_code", Type: core.ConnectionTypeString, Label: "Product Code", Placeholder: "GC1040 - your own catalogue code for this product"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "What the product is, in the words a customer would recognise"},
	{Name: "is_active", Type: core.ConnectionTypeBoolean, Label: "Ready To Sell (leave off and it is still switched on)"},
	{Name: "family", Type: core.ConnectionTypeString, Label: "Product Family", Placeholder: "Must match a value in your org's Product Family list - Salesforce accepts anything else without complaining and stores it as typed"},
	{Name: "quantity_unit_of_measure", Type: core.ConnectionTypeString, Label: "Sold In Units Of", Placeholder: "Each - must match your org's Quantity Unit Of Measure list"},
	{Name: "stock_keeping_unit", Type: core.ConnectionTypeString, Label: "Stock Code (SKU)", Placeholder: "SKU-000142 - the code your warehouse or shop uses"},
	{Name: "display_url", Type: core.ConnectionTypeString, Label: "Product Web Page", Placeholder: "https://www.acme.co.uk/products/genwatt-200kw"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Your Reference", Placeholder: "ERP-000142 - a plain text box on the product; Salesforce does NOT treat it as an External ID, so Create or Update Product cannot match on it"},
	{
		Name:        "product_type",
		Type:        core.ConnectionTypeString,
		Label:       "Bundle Type",
		Placeholder: "Leave blank for an ordinary product",
		Options: []core.ConnectionOption{
			{Name: "Base (an ordinary product)", Value: "Base"},
			{Name: "Bundle (sold as a package of products)", Value: "Bundle"},
			{Name: "Set (a group sold together)", Value: "Set"},
		},
	},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the product"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product ID"},
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

	// Name is the only field Salesforce insists on (verified against the live
	// describe: it is the sole createable, non-nillable field on Product2 apart
	// from IsActive, which has a default). Catching it here names the box the
	// operator left empty instead of relaying REQUIRED_FIELD_MISSING [Name].
	name := salesforce.OptionalString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("the product name is required — what the product is called on quotes and deals, e.g. \"GenWatt Diesel 200kW\"")
	}

	body := map[string]interface{}{"Name": name}

	// Salesforce's own default for Product2.IsActive is FALSE — confirmed on the
	// live describe (defaultValue=false). A product created through the API and
	// left alone is therefore INACTIVE, which means it cannot be given a price,
	// cannot be added to a price book and never appears in the product picker on
	// a deal. Nothing in the response says so: the create returns 201 and the
	// operator finds out days later that "the product is not there".
	//
	// So an untouched tick box means READY TO SELL here, and the label says so.
	// This is the one place the node overrides a Salesforce default, and it is
	// worth it: nobody adds a product to a catalogue in order not to sell it, and
	// an operator who genuinely wants a draft can untick the box.
	if v, set := boolInput("is_active", inputs); set {
		body["IsActive"] = v
	} else {
		body["IsActive"] = true
	}

	salesforce.SetIfPresent(body, inputs, "ProductCode", "product_code")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	// Family and QuantityUnitOfMeasure are UNRESTRICTED picklists in a stock org
	// (restrictedPicklist=false on the live describe, and the Family list ships
	// with nothing in it but "None"). Salesforce accepts an unknown value
	// silently — verified live: Family "Totally Made Up Family" saved without a
	// murmur — so it lands in the customer's catalogue as junk that no report
	// groups on. There is nothing to validate against locally, which is why both
	// placeholders warn instead.
	salesforce.SetIfPresent(body, inputs, "Family", "family")
	salesforce.SetIfPresent(body, inputs, "QuantityUnitOfMeasure", "quantity_unit_of_measure")
	salesforce.SetIfPresent(body, inputs, "StockKeepingUnit", "stock_keeping_unit")
	salesforce.SetIfPresent(body, inputs, "DisplayUrl", "display_url")
	salesforce.SetIfPresent(body, inputs, "ExternalId", "external_id")
	// Type is createable but NOT updateable (live describe), so it is offered
	// here and deliberately absent from Update Product. It is a RESTRICTED
	// picklist, so a wrong value fails loudly rather than silently — the opposite
	// of Family — which is why it can safely be a dropdown.
	salesforce.SetIfPresent(body, inputs, "Type", "product_type")

	// RecordTypeId is deliberately NOT an input. Product2 has no RecordTypeId
	// column at all in a stock org — verified live, a create carrying it fails
	// with INVALID_FIELD "No such column 'RecordTypeId' on sobject of type
	// Product2" — so offering the box would break the action for everyone in
	// order to serve orgs that have switched product record types on. Those orgs
	// can still set it through Additional Fields.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Product2", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Say plainly that the product still has no price. A product with no price
	// book entry cannot be added to a deal, and "Create Product" reading as
	// finished is exactly how an operator ends up stuck at the next step.
	return salesforce.RecordResult(id, raw, fmt.Sprintf(
		"Created product %q (%s) — it has no price yet, so add it to a price book with Add Product to Price Book before putting it on a deal", name, id)), nil
}

// boolInput reads a tick box, reporting separately whether the operator touched
// it. OptionalBool collapses "unticked" and "never touched" into false, which is
// exactly the distinction this action needs in order to default to active.
func boolInput(name string, inputs []*core.Connection) (value, set bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false, false
	}
	return *conn.Boolean(), true
}
