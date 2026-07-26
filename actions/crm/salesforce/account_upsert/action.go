package crm_salesforce_account_upsert

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create or Update Account"
	Description  = "Keep Salesforce in step with another system: match an account on your own reference (an External ID field) and update it if it exists, or create it if it does not. Safe to run over and over — it will not create duplicates."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+rotate"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "external_id_field", Type: core.ConnectionTypeString, Label: "Match Against", Placeholder: "The External ID field to match on, e.g. External_Ref__c", Required: true},
	{Name: "external_id_value", Type: core.ConnectionTypeString, Label: "Match Value", Placeholder: "The value to look for in that field, e.g. CUST-00142", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Acme Manufacturing Ltd — only needed the first time, when the account has to be created"},
	{Name: "account_number", Type: core.ConnectionTypeString, Label: "Account Number", Placeholder: "Your own reference for this customer, e.g. CUST-00142"},
	{Name: "account_type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "Customer, Prospect, Partner — must match a value set up in your org"},
	{Name: "account_source", Type: core.ConnectionTypeString, Label: "Account Source", Placeholder: "Web, Advertisement, Trade Show — must match your org's Account Source list"},
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

	externalIDField := salesforce.OptionalString("external_id_field", inputs)
	if externalIDField == "" {
		return nil, fmt.Errorf("external_id_field is required — the External ID field Salesforce matches on, e.g. External_Ref__c. It must be a field marked External ID or unique in Salesforce Setup")
	}
	// The match field is an identifier, not a value — it goes into the URL path
	// and into a SOQL WHERE clause, neither of which can quote it, so validating
	// it is the only defence. It is also a configuration mistake that never
	// reaches Salesforce, so it takes the hard error return; left to
	// UpsertRecord it would come back on the soft error port instead.
	if _, err := salesforce.ValidateSOQLFieldName(externalIDField); err != nil {
		return nil, fmt.Errorf("external_id_field — %w", err)
	}
	externalIDValue := salesforce.OptionalString("external_id_value", inputs)
	if externalIDValue == "" {
		return nil, fmt.Errorf("external_id_value is required — the value to match, e.g. the customer reference from your other system")
	}
	// Company Name is deliberately NOT required, and is sent only when it is
	// filled in. Forcing it into every payload made a field-level sync — "keep
	// the phone number current, matched on External_Ref__c" — rewrite the account
	// name on every run: an admin tidying "ACME MANUFACTURING LTD" to "Acme
	// Manufacturing Ltd" saw it reverted overnight, silently, with the run
	// reporting success. Salesforce needs a Name only on the half of the upsert
	// that INSERTS, and it asks for it itself (REQUIRED_FIELD_MISSING, translated
	// on the way out) when the match finds nothing — so a partial refresh is
	// expressible and the create case still fails loudly. This is the same
	// reasoning contact_upsert applies to LastName and opportunity_upsert to
	// Name/CloseDate/StageName.
	name := salesforce.OptionalString("name", inputs)
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

	// UpsertRecord path-escapes the match value (external IDs are routinely
	// email addresses or references containing "/") and strips the match field
	// from the body, which Salesforce rejects if it appears in both places.
	id, created, raw, err := salesforce.UpsertRecord(instanceURL, token, "Account", externalIDField, externalIDValue, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	record := raw
	if record == nil {
		record = map[string]interface{}{}
	}
	// An upsert that MATCHED answers 204 No Content, so neither the ID nor the
	// created flag is in the body. Salesforce's own 200/201 response carries a
	// "created" key, so filling it in on the empty case keeps the output shape
	// identical either way — a downstream branch can test it without caring
	// which status came back.
	record["created"] = created
	if id == "" {
		// The 204 case carries no ID at all, so look it up by the same external
		// ID. Without this a matched upsert emits an empty ID and nothing
		// downstream can chain off it — which is the whole point of an upsert
		// in a sync flow. A failure here is not worth failing the upsert over:
		// the record was written, we just could not name it.
		// BuildQueryTyped, not BuildQuery: an External ID field can be a Number
		// as easily as Text, and a numeric external ID quoted as '12345' is a
		// hard INVALID_FIELD — so the literal has to follow the FIELD's type,
		// which one cached describe call settles.
		match := []salesforce.Condition{{Field: externalIDField, Operator: "=", Value: externalIDValue}}
		if soql, qerr := salesforce.BuildQueryTyped(instanceURL, token, "Account", "Id", match, false, "", 1, true); qerr == nil {
			if found, qerr := salesforce.QueryOne(instanceURL, token, soql); qerr == nil && found != nil {
				id = salesforce.StringifyID(found["Id"])
			}
		}
	}
	// Salesforce's own 201 body names the key "id", so fill that key rather
	// than inventing a second one — the raw result then looks the same to a
	// downstream node whether the account was inserted or matched.
	if _, present := record["id"]; !present && id != "" {
		record["id"] = id
	}
	// If even the lookup came back empty, the write still succeeded — but the
	// result would otherwise be an anonymous {"created": false} with nothing in
	// it to say WHICH account was written. The match criteria are the only
	// handle left, and they are what the operator recognises anyway.
	if id == "" {
		record[externalIDField] = externalIDValue
	}

	verb := "Updated"
	if created {
		verb = "Created"
	}
	// Name the account by whatever it is actually called: the box the operator
	// filled in when they filled one in, otherwise whatever Salesforce sent back.
	// Quoting a blank box would read as an account with no name.
	label := name
	if label == "" {
		label, _ = record["Name"].(string)
	}
	summary := fmt.Sprintf("%s account matched on %s = %q", verb, externalIDField, externalIDValue)
	if label != "" {
		summary = fmt.Sprintf("%s account %q matched on %s = %q", verb, label, externalIDField, externalIDValue)
	}
	return salesforce.RecordResult(id, record, summary), nil
}

// applyAccountFields maps the optional inputs onto their Salesforce API names.
// Unset inputs are omitted rather than sent blank, so an upsert that matches an
// existing account only touches the fields the flow actually filled in.
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
