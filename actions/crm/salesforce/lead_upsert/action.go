// Package crm_salesforce_lead_upsert creates a Lead, or updates the existing
// one that matches an external ID.
//
// This is the idempotency primitive for lead automation: it is what stops a
// re-run, a retried webhook or a nightly spreadsheet sync from creating a second
// copy of every lead. Salesforce does the matching server-side in a single
// atomic call, so there is no read-then-decide race for two flow runs to lose.
package crm_salesforce_lead_upsert

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create or Update Lead"
	Description  = "Update the lead that matches an ID from your own system, or create it if there is no match. Safe to re-run — it never makes a duplicate."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+arrow-right-arrow-left"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},

	// The match field must be marked "External ID" (or be a lookup-enabled
	// field such as Email) on the Lead object in Salesforce Setup. A plain text
	// field will not work and Salesforce answers with a fairly opaque error, so
	// the placeholder says so.
	{Name: "external_id_field", Type: core.ConnectionTypeString, Label: "Match On Field", Placeholder: "Your_System_Id__c — must be an External ID field on Lead", Required: true},
	{Name: "external_id_value", Type: core.ConnectionTypeString, Label: "Match Value", Placeholder: "The value to look for, e.g. CRM-10432", Required: true},

	{Name: "company", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Acme Ltd"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Smith"},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Jane"},
	{Name: "salutation", Type: core.ConnectionTypeString, Label: "Salutation", Placeholder: "Mr., Mrs., Ms. or Dr. — must match your org's Salutation list"},
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

	{Name: "status", Type: core.ConnectionTypeString, Label: "Lead Status", Placeholder: "Open - Not Contacted (must match a status in your org)"},
	{Name: "lead_source", Type: core.ConnectionTypeString, Label: "Lead Source", Placeholder: "Web, Trade Show, Employee Referral… — must match your org's Lead Source list"},
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

	externalField, err := salesforce.RequiredString("external_id_field", inputs)
	if err != nil {
		return nil, fmt.Errorf("external_id_field is required — pick the Salesforce field that holds your own system's ID, e.g. Your_System_Id__c")
	}
	// Validate the identifier here rather than letting UpsertRecord reject it
	// deeper down: it is interpolated straight into the request path, and a typo
	// is a configuration mistake — it has to fail hard, not arrive on the error
	// port dressed up as something Salesforce refused. Matches contact_upsert.
	externalField, err = salesforce.ValidateSOQLFieldName(externalField)
	if err != nil {
		return nil, err
	}
	externalValue, err := salesforce.RequiredString("external_id_value", inputs)
	if err != nil {
		return nil, fmt.Errorf("external_id_value is required — it is the value Salesforce matches on")
	}

	body := map[string]interface{}{}
	if err := setLeadFields(body, inputs); err != nil {
		return nil, err
	}
	// Salesforce requires Company and LastName on the INSERT half of an upsert.
	// Refusing an empty body here is a client-side guard: without it the first
	// run of a new sync creates nothing and reports a confusing server error.
	if len(body) == 0 {
		return nil, fmt.Errorf("set at least one lead field — an upsert with no fields has nothing to write. Company Name and Last Name are needed the first time a lead is created")
	}

	// UpsertRecord strips the match field from the body (Salesforce rejects a
	// payload that also sets the field it is matching on) and path-escapes the
	// match value, which matters because an email address or a reference with a
	// "/" in it would otherwise address the wrong URL entirely.
	id, created, raw, err := salesforce.UpsertRecord(instanceURL, token, "Lead", externalField, externalValue, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// An upsert that MATCHED answers 204 No Content with no ID in it. The
	// record does exist, so resolve its ID rather than handing downstream nodes
	// an empty string they cannot chain off.
	if id == "" {
		id = resolveMatchedID(instanceURL, token, externalField, externalValue)
	}

	// Same reasoning for the body. A 204 leaves nothing at all to hand
	// downstream, so echo the ID, the match field and exactly the fields that
	// were sent — those ARE the record's new values. Returning the empty object
	// Salesforce sent would give a downstream node nothing to read, and
	// re-reading the whole lead would spend another call against the org's daily
	// API allowance on every single upsert.
	if len(raw) == 0 {
		raw = map[string]interface{}{"Id": id, externalField: externalValue}
		for field, value := range body {
			raw[field] = value
		}
	}

	verb := "Updated"
	if created {
		verb = "Created"
	}
	summary := fmt.Sprintf("%s lead matched on %s = %s", verb, externalField, externalValue)
	if id != "" {
		summary = fmt.Sprintf("%s lead %s, matched on %s = %s", verb, id, externalField, externalValue)
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

// resolveMatchedID looks up the ID of the lead an upsert just matched.
//
// Best-effort by design: the write already succeeded, so a failure to read the
// ID back must not turn a good run into an error. The caller falls back to an
// empty ID, which is still honest — it just cannot be chained.
//
// The match field is whichever External ID the operator nominated, and it is
// just as often a Number field as a text one — so the value has to be rendered
// against the field's real Salesforce type. Quoting a numeric external ID is a
// hard INVALID_FIELD, which would lose the ID on exactly the orgs that key
// their records numerically. A describe failure degrades to the heuristic.
func resolveMatchedID(instanceURL, token, externalField, externalValue string) string {
	soql, err := salesforce.BuildQueryTyped(instanceURL, token, "Lead", "Id", []salesforce.Condition{
		{Field: externalField, Operator: "=", Value: externalValue},
	}, false, "", 1, true)
	if err != nil {
		return ""
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil || record == nil {
		return ""
	}
	return salesforce.StringifyID(record["Id"])
}

// setLeadFields maps the optional named inputs onto their Salesforce API names.
// Identical mapping to lead_create and lead_update: an upsert is a create or an
// update depending on what Salesforce finds, so the payload must be the same
// either way.
func setLeadFields(body map[string]interface{}, inputs []*core.Connection) error {
	salesforce.SetIfPresent(body, inputs, "Company", "company")
	salesforce.SetIfPresent(body, inputs, "LastName", "last_name")
	salesforce.SetIfPresent(body, inputs, "FirstName", "first_name")
	salesforce.SetIfPresent(body, inputs, "Salutation", "salutation")
	salesforce.SetIfPresent(body, inputs, "Title", "title")
	salesforce.SetIfPresent(body, inputs, "Email", "email")
	salesforce.SetIfPresent(body, inputs, "Phone", "phone")
	salesforce.SetIfPresent(body, inputs, "MobilePhone", "mobile_phone")
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
	salesforce.SetBoolIfSet(body, inputs, "HasOptedOutOfEmail", "has_opted_out_of_email")
	salesforce.SetBoolIfSet(body, inputs, "HasOptedOutOfFax", "has_opted_out_of_fax")
	salesforce.SetBoolIfSet(body, inputs, "IsUnreadByOwner", "is_unread_by_owner")

	if err := setAnnualRevenue(body, inputs); err != nil {
		return err
	}
	return salesforce.MergeAdditionalFields(body, inputs)
}

// setAnnualRevenue writes AnnualRevenue only when the operator supplied a real
// number, failing hard on anything unparseable rather than silently dropping it.
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
