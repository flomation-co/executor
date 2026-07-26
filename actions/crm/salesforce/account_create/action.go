package crm_salesforce_account_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Account"
	Description  = "Add a company to Salesforce as an Account, with its billing and shipping addresses, phone, website, owner and classification. Any field your org has added can be set through Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Acme Manufacturing Ltd", Required: true},
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
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "Salesforce user ID of the account owner, e.g. 0055f000004XyzAAAS"},
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

	name := salesforce.OptionalString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("name is required — Salesforce will not create an account without a company name")
	}

	// Lookup IDs are checked here rather than left to Salesforce: a mistyped
	// owner or parent comes back as MALFORMED_ID, which reads like the provider
	// failed when it is really a typo in the flow.
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

	body := map[string]interface{}{"Name": name}
	applyAccountFields(body, inputs)
	// Every Salesforce org carries custom fields, so the JSON escape hatch is
	// the normal path here, not an edge case. It is merged last so an operator
	// can override anything the named inputs set.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Account", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Created account %q (%s)", name, id)), nil
}

// applyAccountFields maps the optional inputs onto their Salesforce API names.
//
// Every field goes through a Set*IfPresent helper so an untouched input is
// OMITTED from the payload rather than sent blank — the same helper set is used
// by account_update, where sending every empty input would blank half the
// record.
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
	// AnnualRevenue is a Currency field: it must go over the wire as a number,
	// and the float helper is used because the editor hands a decimal amount
	// through as text (the integer reader would truncate 2500000.50).
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
