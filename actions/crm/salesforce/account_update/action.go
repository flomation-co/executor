package crm_salesforce_account_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Account"
	Description  = "Change details on an existing Salesforce account. Only the fields you fill in are changed — everything you leave blank is left exactly as it is."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account", Placeholder: "Record ID of the account to update, e.g. 0015f00000AbCdEAAV", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Acme Manufacturing Ltd"},
	{Name: "account_number", Type: core.ConnectionTypeString, Label: "Account Number", Placeholder: "Your own reference for this customer, e.g. CUST-00142"},
	{Name: "account_type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "Customer, Prospect, Partner — must match a value set up in your org"},
	{Name: "account_source", Type: core.ConnectionTypeString, Label: "Account Source", Placeholder: "Web, Phone Inquiry, Partner Referral"},
	{Name: "industry", Type: core.ConnectionTypeString, Label: "Industry", Placeholder: "Manufacturing, Retail, Education"},
	{Name: "website", Type: core.ConnectionTypeString, Label: "Website", Placeholder: "https://www.acme.co.uk"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "+44 20 7946 0958"},
	{Name: "fax", Type: core.ConnectionTypeString, Label: "Fax", Placeholder: "+44 20 7946 0959"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Notes about this company"},
	{Name: "annual_revenue", Type: core.ConnectionTypeString, Label: "Annual Revenue", Placeholder: "2500000 (in your org's default currency)"},
	{Name: "number_of_employees", Type: core.ConnectionTypeInteger, Label: "Number of Employees", Placeholder: "120"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "Salesforce user ID to transfer the account to, e.g. 0055f000004XyzAAAS"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Parent Account", Placeholder: "Record ID of the parent company, e.g. 0015f00000AbCdEAAV"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Record type ID, only if your org uses account record types"},
	{Name: "sic_desc", Type: core.ConnectionTypeString, Label: "Line of Business", Placeholder: "Short description of what the company does"},
	{Name: "jigsaw", Type: core.ConnectionTypeString, Label: "Data.com Key", Placeholder: "Data.com company ID, if you sync from Data.com"},
	{Name: "billing_street", Type: core.ConnectionTypeString, Label: "Billing Street", Placeholder: "14 Bridge Street"},
	{Name: "billing_city", Type: core.ConnectionTypeString, Label: "Billing City", Placeholder: "Manchester"},
	{Name: "billing_state", Type: core.ConnectionTypeString, Label: "Billing County / State", Placeholder: "California — must match your org's State list, if it uses one"},
	{Name: "billing_postal_code", Type: core.ConnectionTypeString, Label: "Billing Postcode", Placeholder: "M1 4WT"},
	{Name: "billing_country", Type: core.ConnectionTypeString, Label: "Billing Country", Placeholder: "United Kingdom"},
	{Name: "shipping_street", Type: core.ConnectionTypeString, Label: "Shipping Street", Placeholder: "Unit 7, Carlton Industrial Estate"},
	{Name: "shipping_city", Type: core.ConnectionTypeString, Label: "Shipping City", Placeholder: "Leeds"},
	{Name: "shipping_state", Type: core.ConnectionTypeString, Label: "Shipping County / State", Placeholder: "West Yorkshire"},
	{Name: "shipping_postal_code", Type: core.ConnectionTypeString, Label: "Shipping Postcode", Placeholder: "LS1 2AB"},
	{Name: "shipping_country", Type: core.ConnectionTypeString, Label: "Shipping Country", Placeholder: "United Kingdom"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"Rating":"Hot","Customer_Tier__c":"Gold"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Account ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Account"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	accountID := salesforce.OptionalString("account_id", inputs)
	if accountID == "" {
		return nil, fmt.Errorf("account_id is required — the record ID of the account to update, e.g. 0015f00000AbCdEAAV")
	}
	if err := salesforce.ValidateRecordID(accountID); err != nil {
		return nil, err
	}
	for _, lookup := range []struct{ input, label string }{
		{"owner_id", "Owner"},
		{"parent_id", "Parent Account"},
		{"record_type_id", "Record Type"},
	} {
		if v := salesforce.OptionalString(lookup.input, inputs); v != "" {
			if err := salesforce.ValidateRecordID(v); err != nil {
				return nil, fmt.Errorf("%s — %w", lookup.label, err)
			}
		}
	}

	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Name", "name")
	applyAccountFields(body, inputs)
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	// An update with nothing in it would be a silent no-op that still reports
	// success, which is worse than saying so — the operator has almost always
	// forgotten to wire the field they meant to change.
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change, or use Additional Fields")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Account", accountID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	// Salesforce answers a successful PATCH with 204 No Content, so there is no
	// record to echo back. Returning the ID we already hold is what lets the
	// next node in the flow chain off this one.
	// Listing the fields in a stable order keeps the summary readable in a run
	// log, where the same flow is compared across executions.
	changed := salesforce.SortedKeys(body)
	return salesforce.RecordResult(accountID, map[string]interface{}{"Id": accountID}, fmt.Sprintf("Updated account %s — changed %s", accountID, strings.Join(changed, ", "))), nil
}

// applyAccountFields maps the optional inputs onto their Salesforce API names.
//
// Every field goes through a Set*IfPresent helper, which is what makes this a
// genuinely partial update: an untouched input is omitted from the payload
// entirely. Sending it as an empty string would BLANK the field on the record,
// so a half-filled form would quietly wipe an account's address.
func applyAccountFields(body map[string]interface{}, inputs []*core.Connection) {
	salesforce.SetIfPresent(body, inputs, "AccountNumber", "account_number")
	salesforce.SetIfPresent(body, inputs, "Type", "account_type")
	salesforce.SetIfPresent(body, inputs, "AccountSource", "account_source")
	salesforce.SetIfPresent(body, inputs, "Industry", "industry")
	salesforce.SetIfPresent(body, inputs, "Website", "website")
	salesforce.SetIfPresent(body, inputs, "Phone", "phone")
	salesforce.SetIfPresent(body, inputs, "Fax", "fax")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "SicDesc", "sic_desc")
	salesforce.SetIfPresent(body, inputs, "Jigsaw", "jigsaw")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "ParentId", "parent_id")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")
	salesforce.SetFloatIfPresent(body, inputs, "AnnualRevenue", "annual_revenue")
	salesforce.SetIntIfPresent(body, inputs, "NumberOfEmployees", "number_of_employees")
	salesforce.SetIfPresent(body, inputs, "BillingStreet", "billing_street")
	salesforce.SetIfPresent(body, inputs, "BillingCity", "billing_city")
	salesforce.SetIfPresent(body, inputs, "BillingState", "billing_state")
	salesforce.SetIfPresent(body, inputs, "BillingPostalCode", "billing_postal_code")
	salesforce.SetIfPresent(body, inputs, "BillingCountry", "billing_country")
	salesforce.SetIfPresent(body, inputs, "ShippingStreet", "shipping_street")
	salesforce.SetIfPresent(body, inputs, "ShippingCity", "shipping_city")
	salesforce.SetIfPresent(body, inputs, "ShippingState", "shipping_state")
	salesforce.SetIfPresent(body, inputs, "ShippingPostalCode", "shipping_postal_code")
	salesforce.SetIfPresent(body, inputs, "ShippingCountry", "shipping_country")
}
