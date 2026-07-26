package crm_salesforce_asset_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Asset"
	Description  = "Change an asset in Salesforce - mark it Installed once the engineer has been, extend the warranty date, move it to a new owner, retire it as Obsolete. Anything you leave blank is left exactly as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "asset_id", Type: core.ConnectionTypeString, Label: "Asset ID", Placeholder: "02i5f000000AbCdAAK - the asset to change, not its serial number", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Asset Name", Placeholder: "GenWatt Diesel 1000kW - Edge Communications (up to 255 characters)"},
	{Name: "asset_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Shipped, Installed, Registered, Purchased or Obsolete - Salesforce saves anything you type here without complaint, so a typo becomes a real status"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Customer (Account)", Placeholder: "0015f00000AbCdEAAV - do not clear both this and the contact; Salesforce needs one of them"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Customer (Contact)", Placeholder: "0035f00000XyZabAAF - do not clear both this and the account; Salesforce needs one of them"},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - the Salesforce product this asset is one of"},
	{Name: "serial_number", Type: core.ConnectionTypeString, Label: "Serial Number", Placeholder: "SN-00042 - what is stamped on the unit"},
	{Name: "purchase_date", Type: core.ConnectionTypeDateTime, Label: "Purchase Date", Placeholder: "2026-06-20 (the date only)"},
	{Name: "install_date", Type: core.ConnectionTypeDateTime, Label: "Install Date", Placeholder: "2026-07-01 (the date only)"},
	{Name: "usage_end_date", Type: core.ConnectionTypeDateTime, Label: "Warranty / Usage End Date", Placeholder: "2029-06-30 (the date only) - this is the date an \"is it still under warranty?\" check looks at"},
	{Name: "price", Type: core.ConnectionTypeString, Label: "Price", Placeholder: "50000.00 - what the customer paid, in your org's currency"},
	{Name: "quantity", Type: core.ConnectionTypeString, Label: "Quantity", Placeholder: "1 - how many units this record covers"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Part Of (Parent Asset)", Placeholder: "02i5f000000AbCdAAK - if this is a component of a bigger asset"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "0055f00000AbCdEAAV - the Salesforce user who should own the asset"},
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
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other Salesforce field on the asset"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Asset ID"},
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

	id := salesforce.OptionalString("asset_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, fmt.Errorf("Asset ID — %w. A serial number is not a record ID; an asset's record ID starts with 02i", err)
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

	// Every field is optional and every one goes through Set*IfPresent: an update
	// that posted all its blank inputs would clear the operator's data, which on a
	// live asset means wiping the serial number and the warranty date because they
	// only wanted to change the status.
	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Name", "name")
	salesforce.SetIfPresent(body, inputs, "Status", "asset_status")
	salesforce.SetIfPresent(body, inputs, "AccountId", "account_id")
	salesforce.SetIfPresent(body, inputs, "ContactId", "contact_id")
	salesforce.SetIfPresent(body, inputs, "Product2Id", "product_id")
	salesforce.SetIfPresent(body, inputs, "SerialNumber", "serial_number")
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
	// payload entirely, so the asset's existing value stands rather than being
	// silently set to false.
	salesforce.SetBoolIfSet(body, inputs, "IsCompetitorProduct", "is_competitor_product")
	salesforce.SetBoolIfSet(body, inputs, "IsInternal", "is_internal")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "Street", "street")
	salesforce.SetIfPresent(body, inputs, "City", "city")
	salesforce.SetIfPresent(body, inputs, "State", "state")
	salesforce.SetIfPresent(body, inputs, "PostalCode", "postal_code")
	salesforce.SetIfPresent(body, inputs, "Country", "country")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change on the asset")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Asset", id, body); err != nil {
		// FIELD_INTEGRITY_EXCEPTION on this object means one of two very different
		// things, and common.go can only guess at the commoner one (the address
		// State/Country list). Verified live, Asset also raises it for "Every asset
		// needs an account, a contact, or both", which happens here when an update
		// clears the last of the two — so name both possibilities.
		if salesforce.ErrorHasCode(err, "FIELD_INTEGRITY_EXCEPTION") {
			return salesforce.ErrorResult(fmt.Sprintf("Salesforce rejected one of these values — an asset must keep either an account or a contact, and County/State must come from your org's own list rather than being typed by hand (%s)", err.Error())), nil
		}
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf("assets are not available in your Salesforce org — an administrator can switch the Assets tab and object permissions on, and some Salesforce editions do not include them at all (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content — there is no updated record
	// to return. Echo back what was actually applied (plus the ID) so the next node
	// has something to work with and the execution view shows what changed. Use the
	// Get Asset action if the full record is needed.
	changed := salesforce.SortedKeys(body)
	record := make(map[string]interface{}, len(body)+1)
	for k, v := range body {
		record[k] = v
	}
	record["Id"] = id

	return salesforce.RecordResult(id, record, fmt.Sprintf("Updated asset %s — changed %s", id, strings.Join(changed, ", "))), nil
}

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//
