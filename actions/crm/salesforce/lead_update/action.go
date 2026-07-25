// Package crm_salesforce_lead_update changes fields on an existing Lead.
//
// Salesforce PATCH is a partial merge: fields that are not in the payload are
// left exactly as they were, so there is no read-modify-write and no risk of one
// flow clobbering another's edit to a different field. The corollary is that
// blank inputs must be OMITTED, never sent as empty strings — an update that
// posted every unfilled box would wipe half the lead.
package crm_salesforce_lead_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Lead"
	Description  = "Change one or more fields on an existing lead. Anything you leave blank is left exactly as it is in Salesforce."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "lead_id", Type: core.ConnectionTypeString, Label: "Lead ID", Placeholder: "00Q5f000004XyzAEAS", Required: true},

	// Company and Last Name are required to CREATE a lead but are ordinary
	// optional fields on an update, so they sit with everything else here.
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Acme Ltd"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Smith"},
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
	{Name: "state", Type: core.ConnectionTypeString, Label: "County or State", Placeholder: "Greater London"},
	{Name: "postal_code", Type: core.ConnectionTypeString, Label: "Postcode", Placeholder: "SW1A 1AA"},
	{Name: "country", Type: core.ConnectionTypeString, Label: "Country", Placeholder: "United Kingdom"},

	{Name: "status", Type: core.ConnectionTypeString, Label: "Lead Status", Placeholder: "Working - Contacted (must match a status in your org)"},
	{Name: "lead_source", Type: core.ConnectionTypeString, Label: "Lead Source", Placeholder: "Web, Phone Enquiry, Partner Referral…"},
	{Name: "industry", Type: core.ConnectionTypeString, Label: "Industry", Placeholder: "Technology"},
	{Name: "rating", Type: core.ConnectionTypeString, Label: "Rating", Placeholder: "Hot, Warm or Cold"},

	{Name: "annual_revenue", Type: core.ConnectionTypeString, Label: "Annual Revenue", Placeholder: "250000"},
	{Name: "number_of_employees", Type: core.ConnectionTypeInteger, Label: "Number of Employees", Placeholder: "50"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "What the enquiry was about"},
	{Name: "jigsaw", Type: core.ConnectionTypeString, Label: "Data.com Key", Placeholder: "Data.com (Jigsaw) record key, if your org uses it"},

	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Reassign To", Placeholder: "Salesforce user or queue ID, e.g. 0055f000004XyzAAB"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Record type ID, e.g. 0125f000000XyzAAA"},

	{Name: "has_opted_out_of_email", Type: core.ConnectionTypeBoolean, Label: "Opted Out of Email"},
	{Name: "has_opted_out_of_fax", Type: core.ConnectionTypeBoolean, Label: "Opted Out of Fax"},
	{Name: "is_unread_by_owner", Type: core.ConnectionTypeBoolean, Label: "Mark Unread for the Owner"},

	// The escape hatch also carries the only way to CLEAR a field: send it as
	// JSON null, e.g. {"Phone":null}. A blank named input is deliberately
	// treated as "leave alone", so there has to be somewhere to say "erase it".
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Custom_Field__c":"value","Phone":null}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Lead ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Updated Fields"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	leadID, err := salesforce.RequiredString("lead_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("lead_id is required — the Salesforce record ID of the lead to change, e.g. 00Q5f000004XyzAEAS")
	}
	if err := salesforce.ValidateRecordID(leadID); err != nil {
		return nil, err
	}

	body := map[string]interface{}{}
	if err := setLeadFields(body, inputs); err != nil {
		return nil, err
	}
	// Refuse a no-op before spending an API call. Salesforce would answer 204
	// and the run would look successful while having changed nothing — the
	// worst possible outcome for someone debugging a flow that "isn't working".
	if len(body) == 0 {
		return nil, fmt.Errorf("set at least one field to change — an update with no fields would do nothing")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Lead", leadID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers 204 No Content, so there is no record to return. Hand
	// back what was actually written instead of an empty object: it is the only
	// useful thing available and it makes the run history readable.
	changed := salesforce.SortedKeys(body)
	summary := fmt.Sprintf("Updated lead %s (%s)", leadID, strings.Join(changed, ", "))
	return salesforce.RecordResult(leadID, body, summary), nil
}

// setLeadFields maps the optional named inputs onto their Salesforce API names.
// Same mapping as lead_create, deliberately: an operator who learns the field
// names on one action should not have to relearn them on the other.
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

	// n8n never sends these three on update at all — two are read from the
	// wrong key and one is assigned to a lowercase field name. They work here.
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
