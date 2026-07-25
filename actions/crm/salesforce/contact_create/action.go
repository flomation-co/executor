// Package crm_salesforce_contact_create creates a Contact — the person record a
// Salesforce org keeps once an enquiry has become a real relationship, normally
// hanging off an Account. It is the record a front-of-house team touches most,
// so the whole standard field set (name, contact details, both addresses,
// ownership) is exposed as named inputs rather than making the operator
// hand-assemble a JSON body.
package crm_salesforce_contact_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Contact"
	Description  = "Add a new contact to Salesforce. Last Name is the only field Salesforce insists on; link the contact to a company with Account, and use Additional Fields for anything custom to your org."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	// Last Name is required by Salesforce itself, not by us — a Contact without
	// it is rejected server-side. Marking it Required here turns that into an
	// immediate, readable message instead of a round trip that comes back as
	// REQUIRED_FIELD_MISSING.
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Smith", Required: true},

	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Jane"},
	{Name: "middle_name", Type: core.ConnectionTypeString, Label: "Middle Name", Placeholder: "Alexandra (only if your org has middle names switched on)"},
	{Name: "suffix", Type: core.ConnectionTypeString, Label: "Suffix", Placeholder: "Jr, PhD (only if your org has suffixes switched on)"},
	{Name: "salutation", Type: core.ConnectionTypeString, Label: "Salutation", Placeholder: "Mr, Mrs, Ms or Dr"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account", Placeholder: "The company this person works for, e.g. 0015f00000XyzAbAAJ"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Job Title", Placeholder: "Head of Operations"},
	{Name: "department", Type: core.ConnectionTypeString, Label: "Department", Placeholder: "Finance"},

	{Name: "email", Type: core.ConnectionTypeString, Label: "Email Address", Placeholder: "jane.smith@acme.com"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone Number", Placeholder: "+44 20 7946 0958"},
	{Name: "mobile_phone", Type: core.ConnectionTypeString, Label: "Mobile Number", Placeholder: "+44 7700 900123"},
	{Name: "home_phone", Type: core.ConnectionTypeString, Label: "Home Phone", Placeholder: "+44 20 7946 0100"},
	{Name: "other_phone", Type: core.ConnectionTypeString, Label: "Other Phone", Placeholder: "+44 20 7946 0200"},
	{Name: "fax", Type: core.ConnectionTypeString, Label: "Fax Number", Placeholder: "+44 20 7946 0999"},

	{Name: "mailing_street", Type: core.ConnectionTypeString, Label: "Mailing Street", Placeholder: "12 High Street"},
	{Name: "mailing_city", Type: core.ConnectionTypeString, Label: "Mailing Town or City", Placeholder: "London"},
	{Name: "mailing_state", Type: core.ConnectionTypeString, Label: "Mailing County or State", Placeholder: "Greater London"},
	{Name: "mailing_postal_code", Type: core.ConnectionTypeString, Label: "Mailing Postcode", Placeholder: "SW1A 1AA"},
	{Name: "mailing_country", Type: core.ConnectionTypeString, Label: "Mailing Country", Placeholder: "United Kingdom"},

	{Name: "other_street", Type: core.ConnectionTypeString, Label: "Other Street", Placeholder: "Second address, e.g. 4 Mill Lane"},
	{Name: "other_city", Type: core.ConnectionTypeString, Label: "Other Town or City", Placeholder: "Manchester"},
	{Name: "other_state", Type: core.ConnectionTypeString, Label: "Other County or State", Placeholder: "Greater Manchester"},
	{Name: "other_postal_code", Type: core.ConnectionTypeString, Label: "Other Postcode", Placeholder: "M1 2AB"},
	{Name: "other_country", Type: core.ConnectionTypeString, Label: "Other Country", Placeholder: "United Kingdom"},

	// Lead Source is a picklist and every org edits its own list. Plain text
	// here: the value must match one of your org's options exactly or Salesforce
	// rejects it.
	{Name: "lead_source", Type: core.ConnectionTypeString, Label: "Lead Source", Placeholder: "Web, Phone Enquiry, Partner Referral…"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Background on this person"},

	{Name: "assistant_name", Type: core.ConnectionTypeString, Label: "Assistant Name", Placeholder: "Chris Taylor"},
	{Name: "assistant_phone", Type: core.ConnectionTypeString, Label: "Assistant Phone", Placeholder: "+44 20 7946 0300"},

	// Birthdate is a DATE field in Salesforce, so a full timestamp is rejected
	// outright. SetDateIfPresent trims whatever the date picker hands over down
	// to YYYY-MM-DD.
	{Name: "birthdate", Type: core.ConnectionTypeDateTime, Label: "Date of Birth"},

	{Name: "email_bounced_date", Type: core.ConnectionTypeDateTime, Label: "Email Bounced Date"},
	{Name: "email_bounced_reason", Type: core.ConnectionTypeString, Label: "Email Bounced Reason", Placeholder: "Mailbox full"},
	{Name: "has_opted_out_of_email", Type: core.ConnectionTypeBoolean, Label: "Opted Out of Email"},

	{Name: "pronouns", Type: core.ConnectionTypeString, Label: "Pronouns", Placeholder: "She/Her (only if your org has Pronouns switched on)"},
	{Name: "gender_identity", Type: core.ConnectionTypeString, Label: "Gender Identity", Placeholder: "Only if your org has Gender Identity switched on"},
	{Name: "jigsaw", Type: core.ConnectionTypeString, Label: "Data.com Key", Placeholder: "Data.com (Jigsaw) record key, if your org uses it"},

	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Assign To", Placeholder: "Salesforce user ID, e.g. 0055f000004XyzAAB"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Record type ID, e.g. 0125f000000XyzAAA"},

	// Every Salesforce org has custom fields, so the escape hatch is the normal
	// path here, not an edge case. Keys are Salesforce API names.
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Custom_Field__c":"value","Region__c":"South"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
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

	lastName, err := salesforce.RequiredString("last_name", inputs)
	if err != nil {
		return nil, fmt.Errorf("last_name is required — Salesforce will not accept a contact without a surname")
	}

	body := map[string]interface{}{"LastName": lastName}
	if err := setContactFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Contact", body)
	if err != nil {
		// Anything Salesforce rejected (a validation rule, a duplicate rule, an
		// account ID that does not exist) is a provider failure, so it lands on
		// the error port as data rather than killing the flow.
		return salesforce.ErrorResult(err.Error()), nil
	}

	name := lastName
	if first := salesforce.OptionalString("first_name", inputs); first != "" {
		name = first + " " + lastName
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Created contact %s (%s)", name, id)), nil
}

// setContactFields maps the optional named inputs onto their Salesforce API
// names.
//
// Every write goes through Set*IfPresent so an input the operator left blank is
// OMITTED from the payload rather than sent as an empty string — Salesforce
// treats an omitted field and an explicitly-blank one differently, and the
// distinction matters far more on update than create.
//
// n8n hand-writes this mapping as a ~35-line if-chain and duplicates it between
// its create and update branches, which is precisely how two of its contact
// fields ended up unreachable: Assistant Phone is declared as "Assistant Phone"
// but read as assistantPhone (so it is never sent at all), and on create Email
// Bounced Date is declared under the name otherPostalCode, so it silently
// overwrites the Other Postcode instead. Both are correct here. The same
// function is restated in contact_upsert and contact_update — one action per
// package means it cannot be shared, so keep the three in step.
func setContactFields(body map[string]interface{}, inputs []*core.Connection) error {
	salesforce.SetIfPresent(body, inputs, "FirstName", "first_name")
	salesforce.SetIfPresent(body, inputs, "MiddleName", "middle_name")
	salesforce.SetIfPresent(body, inputs, "Suffix", "suffix")
	salesforce.SetIfPresent(body, inputs, "Salutation", "salutation")
	salesforce.SetIfPresent(body, inputs, "AccountId", "account_id")
	salesforce.SetIfPresent(body, inputs, "Title", "title")
	salesforce.SetIfPresent(body, inputs, "Department", "department")

	salesforce.SetIfPresent(body, inputs, "Email", "email")
	salesforce.SetIfPresent(body, inputs, "Phone", "phone")
	salesforce.SetIfPresent(body, inputs, "MobilePhone", "mobile_phone")
	salesforce.SetIfPresent(body, inputs, "HomePhone", "home_phone")
	salesforce.SetIfPresent(body, inputs, "OtherPhone", "other_phone")
	// Fax is a text field on Salesforce — "+44 (0)20 7946 0999" is a perfectly
	// ordinary fax number and a numeric input would mangle it.
	salesforce.SetIfPresent(body, inputs, "Fax", "fax")

	salesforce.SetIfPresent(body, inputs, "MailingStreet", "mailing_street")
	salesforce.SetIfPresent(body, inputs, "MailingCity", "mailing_city")
	salesforce.SetIfPresent(body, inputs, "MailingState", "mailing_state")
	salesforce.SetIfPresent(body, inputs, "MailingPostalCode", "mailing_postal_code")
	salesforce.SetIfPresent(body, inputs, "MailingCountry", "mailing_country")

	salesforce.SetIfPresent(body, inputs, "OtherStreet", "other_street")
	salesforce.SetIfPresent(body, inputs, "OtherCity", "other_city")
	salesforce.SetIfPresent(body, inputs, "OtherState", "other_state")
	salesforce.SetIfPresent(body, inputs, "OtherPostalCode", "other_postal_code")
	salesforce.SetIfPresent(body, inputs, "OtherCountry", "other_country")

	salesforce.SetIfPresent(body, inputs, "LeadSource", "lead_source")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "AssistantName", "assistant_name")
	salesforce.SetIfPresent(body, inputs, "AssistantPhone", "assistant_phone")

	// Birthdate is a Date field and EmailBouncedDate is a DateTime one, which is
	// why only the first is trimmed to YYYY-MM-DD. Sending a full timestamp to a
	// Date field fails with a malformed-date error.
	salesforce.SetDateIfPresent(body, inputs, "Birthdate", "birthdate")
	salesforce.SetIfPresent(body, inputs, "EmailBouncedDate", "email_bounced_date")
	salesforce.SetIfPresent(body, inputs, "EmailBouncedReason", "email_bounced_reason")

	salesforce.SetIfPresent(body, inputs, "Pronouns", "pronouns")
	salesforce.SetIfPresent(body, inputs, "GenderIdentity", "gender_identity")
	salesforce.SetIfPresent(body, inputs, "Jigsaw", "jigsaw")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")

	// SetBoolIfSet, not a truthiness test, so an explicit "false" is
	// transmitted. n8n drops false here, which makes it impossible to clear an
	// opt-out flag once it is set.
	salesforce.SetBoolIfSet(body, inputs, "HasOptedOutOfEmail", "has_opted_out_of_email")

	// Additional fields go on last so a custom value deliberately wins over a
	// named input set to the same API name.
	return salesforce.MergeAdditionalFields(body, inputs)
}
