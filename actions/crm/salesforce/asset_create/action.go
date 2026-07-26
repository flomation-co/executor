package crm_salesforce_asset_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Asset"
	Description  = "Record something a customer owns - which product it is, its serial number, when it was installed and when the warranty runs out. It is what turns \"they bought a boiler two years ago\" into something the person answering the phone can actually look up."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+box"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Asset Name", Placeholder: "GenWatt Diesel 1000kW - Edge Communications (up to 255 characters)", Required: true},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Customer (Account)", Placeholder: "0015f00000AbCdEAAV - every asset needs an account, a contact, or both"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Customer (Contact)", Placeholder: "0035f00000XyZabAAF - every asset needs an account, a contact, or both"},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - the Salesforce product this asset is one of; its product code is copied across for you"},
	{Name: "serial_number", Type: core.ConnectionTypeString, Label: "Serial Number", Placeholder: "SN-00042 - what is stamped on the unit"},
	{Name: "asset_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Shipped, Installed, Registered, Purchased or Obsolete - Salesforce saves anything you type here without complaint, so a typo becomes a real status"},
	{Name: "purchase_date", Type: core.ConnectionTypeDateTime, Label: "Purchase Date", Placeholder: "2026-06-20 (the date only)"},
	{Name: "install_date", Type: core.ConnectionTypeDateTime, Label: "Install Date", Placeholder: "2026-07-01 (the date only)"},
	{Name: "usage_end_date", Type: core.ConnectionTypeDateTime, Label: "Warranty / Usage End Date", Placeholder: "2029-06-30 (the date only) - this is the date an \"is it still under warranty?\" check looks at"},
	{Name: "price", Type: core.ConnectionTypeString, Label: "Price", Placeholder: "50000.00 - what the customer paid, in your org's currency"},
	{Name: "quantity", Type: core.ConnectionTypeString, Label: "Quantity", Placeholder: "1 - how many units this record covers"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Part Of (Parent Asset)", Placeholder: "02i5f000000AbCdAAK - if this is a component of a bigger asset"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "0055f00000AbCdEAAV - leave blank and the connected Salesforce user owns it"},
	{Name: "asset_provided_by_id", Type: core.ConnectionTypeString, Label: "Supplied By (Account)", Placeholder: "0015f00000AbCdEAAV - the company that supplied it, if it was not you"},
	{Name: "asset_serviced_by_id", Type: core.ConnectionTypeString, Label: "Serviced By (Account)", Placeholder: "0015f00000AbCdEAAV - the company that maintains it"},
	{Name: "is_competitor_product", Type: core.ConnectionTypeBoolean, Label: "This Is A Competitor's Product"},
	{Name: "is_internal", Type: core.ConnectionTypeBoolean, Label: "This Is Your Own Company's Asset"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Anything the support team should know - where it is on site, who signed for it"},
	{Name: "street", Type: core.ConnectionTypeText, Label: "Location Street", Placeholder: "1 Deansgate - where the asset physically is"},
	{Name: "city", Type: core.ConnectionTypeString, Label: "Location City", Placeholder: "Manchester"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "Location County / State", Placeholder: "Leave this blank unless your org's State list has an entry for it - most orgs check it against a fixed list and refuse anything typed by hand"},
	{Name: "postal_code", Type: core.ConnectionTypeString, Label: "Location Postcode", Placeholder: "M1 1AA"},
	{Name: "country", Type: core.ConnectionTypeString, Label: "Location Country", Placeholder: "United Kingdom"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "0125f000000AbCdAAK - only if your org uses record types on assets"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other Salesforce field on the asset"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Asset ID"},
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

	name := salesforce.OptionalString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("the asset name is required — what this piece of kit is called in Salesforce, e.g. \"GenWatt Diesel 1000kW - Edge Communications\"")
	}

	accountID := salesforce.OptionalString("account_id", inputs)
	contactID := salesforce.OptionalString("contact_id", inputs)
	// Salesforce enforces this one itself, and its answer is genuinely misleading:
	// FIELD_INTEGRITY_EXCEPTION, which common.go translates as an address
	// State/Country problem because that is what the code means everywhere else.
	// Verified live, the message is "Every asset needs an account, a contact, or
	// both." Catching it here sends the operator to the right box.
	if accountID == "" && contactID == "" {
		return nil, fmt.Errorf("choose who owns this asset — Salesforce needs an account, a contact, or both, and refuses the record with a confusing \"field integrity\" error otherwise")
	}

	// Each of these is a lookup to another record. A typo produces
	// INVALID_CROSS_REFERENCE_KEY, which does not say WHICH box was wrong, so the
	// shape check happens here where the field name is still known.
	for _, ref := range []struct{ label, name string }{
		{"Customer (Account)", "account_id"},
		{"Customer (Contact)", "contact_id"},
		{"Product", "product_id"},
		{"Part Of (Parent Asset)", "parent_id"},
		{"Owner", "owner_id"},
		{"Supplied By (Account)", "asset_provided_by_id"},
		{"Serviced By (Account)", "asset_serviced_by_id"},
	} {
		if v := salesforce.OptionalString(ref.name, inputs); v != "" {
			if err := salesforce.ValidateRecordID(v); err != nil {
				return nil, fmt.Errorf("%s — %w", ref.label, err)
			}
		}
	}

	body := map[string]interface{}{"Name": name}
	salesforce.SetIfPresent(body, inputs, "AccountId", "account_id")
	salesforce.SetIfPresent(body, inputs, "ContactId", "contact_id")
	salesforce.SetIfPresent(body, inputs, "Product2Id", "product_id")
	salesforce.SetIfPresent(body, inputs, "SerialNumber", "serial_number")
	salesforce.SetIfPresent(body, inputs, "Status", "asset_status")
	// PurchaseDate, InstallDate and UsageEndDate are Date fields, not Date/Time —
	// a date-picker upstream hands over a full ISO timestamp, so trim it.
	salesforce.SetDateIfPresent(body, inputs, "PurchaseDate", "purchase_date")
	salesforce.SetDateIfPresent(body, inputs, "InstallDate", "install_date")
	salesforce.SetDateIfPresent(body, inputs, "UsageEndDate", "usage_end_date")
	price, priceSet, err := salesforce.NumericInput("price", "Price", "50000.00", inputs)
	if err != nil {
		return nil, err
	}
	if priceSet {
		body["Price"] = price
	}
	quantity, quantitySet, err := salesforce.NumericInput("quantity", "Quantity", "50000.00", inputs)
	if err != nil {
		return nil, err
	}
	if quantitySet {
		body["Quantity"] = quantity
	}
	salesforce.SetIfPresent(body, inputs, "ParentId", "parent_id")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "AssetProvidedById", "asset_provided_by_id")
	salesforce.SetIfPresent(body, inputs, "AssetServicedById", "asset_serviced_by_id")
	// SetBoolIfSet, not SetIfPresent: an untouched tick box stays out of the
	// payload entirely, so Salesforce's own default stands.
	salesforce.SetBoolIfSet(body, inputs, "IsCompetitorProduct", "is_competitor_product")
	salesforce.SetBoolIfSet(body, inputs, "IsInternal", "is_internal")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "Street", "street")
	salesforce.SetIfPresent(body, inputs, "City", "city")
	salesforce.SetIfPresent(body, inputs, "State", "state")
	salesforce.SetIfPresent(body, inputs, "PostalCode", "postal_code")
	salesforce.SetIfPresent(body, inputs, "Country", "country")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")

	// Every org has custom fields on Asset — contract reference, site code, the
	// engineer who fitted it — and none of them can be a first-class input here.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Asset", body)
	if err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf("assets are not available in your Salesforce org — an administrator can switch the Assets tab and object permissions on, and some Salesforce editions do not include them at all (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	owner := accountID
	if owner == "" {
		owner = contactID
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Created asset %q for %s (%s)", name, owner, id)), nil
}

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//
