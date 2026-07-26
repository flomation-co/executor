// Package crm_salesforce_contact_update changes fields on an existing Contact.
//
// Salesforce updates are partial: only the fields actually sent are touched.
// Every input therefore goes through Set*IfPresent, so leaving a box empty means
// "leave this alone" rather than "blank it" — an update that transmitted every
// unfilled input would wipe half the record on the first run.
package crm_salesforce_contact_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Contact"
	Description  = "Change one or more fields on an existing contact. Anything you leave blank is left exactly as it is in Salesforce, so you can update a single phone number without touching the rest."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "0035f00000XyzAbAAJ — from the contact's Salesforce URL", Required: true},

	// Last Name is required to CREATE a contact but not to update one, so it is
	// an ordinary optional input here.
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Smith"},
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
	{Name: "mailing_state", Type: core.ConnectionTypeString, Label: "Mailing County or State", Placeholder: "California — must match your org's State list, if it uses one"},
	{Name: "mailing_postal_code", Type: core.ConnectionTypeString, Label: "Mailing Postcode", Placeholder: "SW1A 1AA"},
	{Name: "mailing_country", Type: core.ConnectionTypeString, Label: "Mailing Country", Placeholder: "United Kingdom"},

	{Name: "other_street", Type: core.ConnectionTypeString, Label: "Other Street", Placeholder: "Second address, e.g. 4 Mill Lane"},
	{Name: "other_city", Type: core.ConnectionTypeString, Label: "Other Town or City", Placeholder: "Manchester"},
	{Name: "other_state", Type: core.ConnectionTypeString, Label: "Other County or State", Placeholder: "California — must match your org's State list, if it uses one"},
	{Name: "other_postal_code", Type: core.ConnectionTypeString, Label: "Other Postcode", Placeholder: "M1 2AB"},
	{Name: "other_country", Type: core.ConnectionTypeString, Label: "Other Country", Placeholder: "United Kingdom"},

	{Name: "lead_source", Type: core.ConnectionTypeString, Label: "Lead Source", Placeholder: "Web, Phone Enquiry, Partner Referral…"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Background on this person"},

	{Name: "assistant_name", Type: core.ConnectionTypeString, Label: "Assistant Name", Placeholder: "Chris Taylor"},
	{Name: "assistant_phone", Type: core.ConnectionTypeString, Label: "Assistant Phone", Placeholder: "+44 20 7946 0300"},

	{Name: "birthdate", Type: core.ConnectionTypeDateTime, Label: "Date of Birth"},
	{Name: "email_bounced_date", Type: core.ConnectionTypeDateTime, Label: "Email Bounced Date"},
	{Name: "email_bounced_reason", Type: core.ConnectionTypeString, Label: "Email Bounced Reason", Placeholder: "Mailbox full"},
	{Name: "has_opted_out_of_email", Type: core.ConnectionTypeBoolean, Label: "Opted Out of Email"},

	{Name: "pronouns", Type: core.ConnectionTypeString, Label: "Pronouns", Placeholder: "She/Her (only if your org has Pronouns switched on)"},
	{Name: "gender_identity", Type: core.ConnectionTypeString, Label: "Gender Identity", Placeholder: "Only if your org has Gender Identity switched on"},
	{Name: "jigsaw", Type: core.ConnectionTypeString, Label: "Data.com Key", Placeholder: "Data.com (Jigsaw) record key, if your org uses it"},

	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Assign To", Placeholder: "Salesforce user ID, e.g. 0055f000004XyzAAB"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Record type ID, e.g. 0125f000000XyzAAA"},

	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Custom_Field__c":"value","Region__c":"South"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
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

	contactID, err := salesforce.RequiredString("contact_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("contact_id is required — the 15 or 18 character ID from the contact's Salesforce URL")
	}
	if err := salesforce.ValidateRecordID(contactID); err != nil {
		return nil, err
	}

	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "LastName", "last_name")
	if err := setContactFields(body, inputs); err != nil {
		return nil, err
	}
	// An update with nothing to change is a configuration mistake, and catching
	// it here spends no API call and no time on the org's daily allowance.
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field, or add the field you want under Additional Fields")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Contact", contactID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers a successful update with 204 No Content, so there is no
	// record to return. Echo the ID plus exactly the fields that were sent
	// rather than re-reading the contact: a second API call per update would
	// double the cost of every update in a loop against the org's daily
	// allowance, and those values ARE the record's new values.
	record := map[string]interface{}{"Id": contactID}
	for field, value := range body {
		record[field] = value
	}

	// SortedKeys keeps the field list stable, so the same update reads
	// identically in the execution view on every run.
	return salesforce.RecordResult(contactID, record,
		fmt.Sprintf("Updated contact %s — changed %d field(s): %s", contactID, len(body), strings.Join(salesforce.SortedKeys(body), ", "))), nil
}

// setContactFields maps the optional named inputs onto their Salesforce API
// names. See contact_create for the full rationale; the same function is
// restated in contact_create and contact_upsert, so keep the three in step.
//
// Last Name is handled by the caller, because it is a required top-level input
// on create and an optional one on upsert and update.
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
	// why only the first is trimmed to YYYY-MM-DD.
	salesforce.SetDateIfPresent(body, inputs, "Birthdate", "birthdate")
	salesforce.SetIfPresent(body, inputs, "EmailBouncedDate", "email_bounced_date")
	salesforce.SetIfPresent(body, inputs, "EmailBouncedReason", "email_bounced_reason")

	salesforce.SetIfPresent(body, inputs, "Pronouns", "pronouns")
	salesforce.SetIfPresent(body, inputs, "GenderIdentity", "gender_identity")
	salesforce.SetIfPresent(body, inputs, "Jigsaw", "jigsaw")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")

	salesforce.SetBoolIfSet(body, inputs, "HasOptedOutOfEmail", "has_opted_out_of_email")

	return salesforce.MergeAdditionalFields(body, inputs)
}
