// Package crm_salesforce_lead_create creates a Lead — the record a Salesforce
// org uses for an unqualified enquiry, before anyone has decided it is worth a
// real Account and Contact. It is the entry point for almost every inbound
// automation a front-of-house team builds ("web form submitted → create a lead
// and tell the sales inbox"), which is why the whole standard field set is
// exposed as named inputs rather than making the operator hand-assemble JSON.
package crm_salesforce_lead_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Lead"
	Description  = "Add a new lead to Salesforce from a web form, phone enquiry or spreadsheet row. Company Name and Last Name are the only fields Salesforce insists on; everything else is optional."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	// Company and Last Name are required by Salesforce itself, not by us — a
	// Lead without them is rejected server-side. Marking them Required here
	// turns that into an immediate, readable message instead of a round trip
	// that comes back as REQUIRED_FIELD_MISSING.
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Acme Ltd", Required: true},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Smith", Required: true},

	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Jane"},
	{Name: "salutation", Type: core.ConnectionTypeString, Label: "Salutation", Placeholder: "Mr, Mrs, Ms or Dr"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Job Title", Placeholder: "Head of Operations"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email Address", Placeholder: "jane.smith@acme.com"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone Number", Placeholder: "+44 20 7946 0958"},
	{Name: "mobile_phone", Type: core.ConnectionTypeString, Label: "Mobile Number", Placeholder: "+44 7700 900123"},
	{Name: "fax", Type: core.ConnectionTypeString, Label: "Fax Number", Placeholder: "+44 20 7946 0999"},
	{Name: "website", Type: core.ConnectionTypeString, Label: "Website", Placeholder: "https://www.acme.com"},

	{Name: "street", Type: core.ConnectionTypeString, Label: "Street", Placeholder: "12 High Street"},
	{Name: "city", Type: core.ConnectionTypeString, Label: "Town or City", Placeholder: "London"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "County or State", Placeholder: "California — must match your org's State list, if it uses one"},
	{Name: "postal_code", Type: core.ConnectionTypeString, Label: "Postcode", Placeholder: "SW1A 1AA"},
	{Name: "country", Type: core.ConnectionTypeString, Label: "Country", Placeholder: "United Kingdom"},

	// Status, Lead Source, Industry and Rating are picklists in Salesforce, and
	// every org edits its own list. They are plain text here: the value must
	// match one of your org's options exactly or Salesforce rejects it.
	{Name: "status", Type: core.ConnectionTypeString, Label: "Lead Status", Placeholder: "Open - Not Contacted (must match a status in your org)"},
	{Name: "lead_source", Type: core.ConnectionTypeString, Label: "Lead Source", Placeholder: "Web, Phone Enquiry, Partner Referral…"},
	{Name: "industry", Type: core.ConnectionTypeString, Label: "Industry", Placeholder: "Technology"},
	{Name: "rating", Type: core.ConnectionTypeString, Label: "Rating", Placeholder: "Hot, Warm or Cold"},

	{Name: "annual_revenue", Type: core.ConnectionTypeString, Label: "Annual Revenue", Placeholder: "250000"},
	{Name: "number_of_employees", Type: core.ConnectionTypeInteger, Label: "Number of Employees", Placeholder: "50"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "What the enquiry was about"},
	{Name: "jigsaw", Type: core.ConnectionTypeString, Label: "Data.com Key", Placeholder: "Data.com (Jigsaw) record key, if your org uses it"},

	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Assign To", Placeholder: "Salesforce user or queue ID, e.g. 0055f000004XyzAAB"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Record type ID, e.g. 0125f000000XyzAAA"},

	{Name: "has_opted_out_of_email", Type: core.ConnectionTypeBoolean, Label: "Opted Out of Email"},
	{Name: "has_opted_out_of_fax", Type: core.ConnectionTypeBoolean, Label: "Opted Out of Fax"},
	{Name: "is_unread_by_owner", Type: core.ConnectionTypeBoolean, Label: "Mark Unread for the Owner"},

	// Every Salesforce org has custom fields, so the escape hatch is the normal
	// path here, not an edge case. Keys are Salesforce API names.
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Custom_Field__c":"value","Region__c":"South"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Lead ID"},
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

	company, err := salesforce.RequiredString("company", inputs)
	if err != nil {
		return nil, fmt.Errorf("company is required — Salesforce will not accept a lead without a company name")
	}
	lastName, err := salesforce.RequiredString("last_name", inputs)
	if err != nil {
		return nil, fmt.Errorf("last_name is required — Salesforce will not accept a lead without a surname")
	}

	body := map[string]interface{}{
		"Company":  company,
		"LastName": lastName,
	}
	if err := setLeadFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Lead", body)
	if err != nil {
		// Anything Salesforce rejected (a validation rule, a duplicate rule, a
		// picklist value the org does not have) is a provider failure, so it
		// lands on the error port as data rather than killing the flow.
		return salesforce.ErrorResult(err.Error()), nil
	}

	name := lastName
	if first := salesforce.OptionalString("first_name", inputs); first != "" {
		name = first + " " + lastName
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Created lead %s at %s (%s)", name, company, id)), nil
}

// setLeadFields maps the optional named inputs onto their Salesforce API names.
//
// Every write goes through Set*IfPresent so an input the operator left blank is
// OMITTED from the payload rather than sent as an empty string — Salesforce
// treats an omitted field and an explicitly-blank one differently, and the
// distinction matters far more on update than create. Keeping create and update
// on the identical mapping is what stops the two drifting apart.
func setLeadFields(body map[string]interface{}, inputs []*core.Connection) error {
	salesforce.SetIfPresent(body, inputs, "FirstName", "first_name")
	salesforce.SetIfPresent(body, inputs, "Salutation", "salutation")
	salesforce.SetIfPresent(body, inputs, "Title", "title")
	salesforce.SetIfPresent(body, inputs, "Email", "email")
	salesforce.SetIfPresent(body, inputs, "Phone", "phone")
	salesforce.SetIfPresent(body, inputs, "MobilePhone", "mobile_phone")
	// Fax is a text field on Salesforce despite n8n typing it as a number —
	// "+44 (0)20 7946 0999" is a perfectly ordinary fax number and a numeric
	// input would mangle it.
	salesforce.SetIfPresent(body, inputs, "Fax", "fax")
	salesforce.SetIfPresent(body, inputs, "Website", "website")
	salesforce.SetIfPresent(body, inputs, "Street", "street")
	salesforce.SetIfPresent(body, inputs, "City", "city")
	salesforce.SetIfPresent(body, inputs, "State", "state")
	salesforce.SetIfPresent(body, inputs, "PostalCode", "postal_code")
	salesforce.SetIfPresent(body, inputs, "Country", "country")
	salesforce.SetIfPresent(body, inputs, "Status", "status")
	salesforce.SetIfPresent(body, inputs, "LeadSource", "lead_source")
	salesforce.SetIfPresent(body, inputs, "Industry", "industry")
	salesforce.SetIfPresent(body, inputs, "Rating", "rating")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "Jigsaw", "jigsaw")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")
	salesforce.SetIntIfPresent(body, inputs, "NumberOfEmployees", "number_of_employees")

	// The three checkboxes use SetBoolIfSet, not a truthiness test, so an
	// explicit "false" is transmitted. n8n drops false here, which makes it
	// impossible to clear an opt-out flag once it is set.
	salesforce.SetBoolIfSet(body, inputs, "HasOptedOutOfEmail", "has_opted_out_of_email")
	salesforce.SetBoolIfSet(body, inputs, "HasOptedOutOfFax", "has_opted_out_of_fax")
	salesforce.SetBoolIfSet(body, inputs, "IsUnreadByOwner", "is_unread_by_owner")

	if err := setAnnualRevenue(body, inputs); err != nil {
		return err
	}
	// Additional fields go on last so a custom value deliberately wins over a
	// named input set to the same API name.
	return salesforce.MergeAdditionalFields(body, inputs)
}

// setAnnualRevenue writes AnnualRevenue only when the operator supplied a real
// number.
//
// SetFloatIfPresent on its own would silently DROP "£250,000" or "250k" — the
// lead would be created with no revenue and nobody would know why. A value that
// was typed but cannot be parsed is a configuration mistake, so it fails hard.
func setAnnualRevenue(body map[string]interface{}, inputs []*core.Connection) error {
	raw := salesforce.OptionalString("annual_revenue", inputs)
	if raw == "" {
		return nil
	}
	v, ok := salesforce.OptionalFloat("annual_revenue", inputs)
	if !ok {
		return fmt.Errorf("annual_revenue must be a plain number with no currency symbol or thousands separators, e.g. 250000 — got %q", raw)
	}
	body["AnnualRevenue"] = v
	return nil
}
