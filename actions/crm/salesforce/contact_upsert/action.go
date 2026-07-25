// Package crm_salesforce_contact_upsert creates a Contact, or updates the
// existing one that matches, in a single call.
//
// This is the idempotency primitive for contact automation: a flow that syncs a
// mailing list or replays a webhook can run twice without producing a second
// copy of every person, because Salesforce matches on an external ID field
// (usually Email) instead of a record ID nobody outside Salesforce has.
package crm_salesforce_contact_upsert

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create or Update Contact"
	Description  = "Create a contact, or update the one that already matches. Salesforce matches on a field you choose — usually Email — so re-running a flow updates the same person instead of adding a duplicate."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+arrow-right-arrow-left"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	// The match pair. Salesforce will only match on a field marked External ID
	// or Lookup in the org — Email and Id qualify out of the box, anything else
	// has to be set up by an administrator.
	{Name: "external_id_field", Type: core.ConnectionTypeString, Label: "Match On", Placeholder: "Email — or another External ID field on Contact", Required: true},
	{Name: "external_id_value", Type: core.ConnectionTypeString, Label: "Match Value", Placeholder: "jane.smith@acme.com — the value to look for", Required: true},

	// Deliberately NOT required, unlike n8n: an upsert that matches an existing
	// person does not need a surname, and forcing one would overwrite whatever
	// Salesforce already holds. Salesforce asks for it only when the record is
	// genuinely new, and says so plainly if it is missing.
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Smith — needed only if this creates a new contact"},

	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Jane"},
	{Name: "middle_name", Type: core.ConnectionTypeString, Label: "Middle Name", Placeholder: "Alexandra (only if your org has middle names switched on)"},
	{Name: "suffix", Type: core.ConnectionTypeString, Label: "Suffix", Placeholder: "Jr, PhD (only if your org has suffixes switched on)"},
	{Name: "salutation", Type: core.ConnectionTypeString, Label: "Salutation", Placeholder: "Mr., Mrs., Ms. or Dr. — must match your org's Salutation list"},
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

	{Name: "lead_source", Type: core.ConnectionTypeString, Label: "Lead Source", Placeholder: "Web, Trade Show, Advertisement — must match your org's Lead Source list"},
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

	extField, err := salesforce.RequiredString("external_id_field", inputs)
	if err != nil {
		return nil, fmt.Errorf("external_id_field is required — the Salesforce field to match on, e.g. Email")
	}
	// Validate the identifier here rather than letting it reach the URL: it is
	// interpolated into the request path, and a typo should read as a
	// configuration mistake, not as a Salesforce 404.
	extField, err = salesforce.ValidateSOQLFieldName(extField)
	if err != nil {
		return nil, err
	}
	extValue, err := salesforce.RequiredString("external_id_value", inputs)
	if err != nil {
		return nil, fmt.Errorf("external_id_value is required — it is the value Salesforce matches on")
	}

	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "LastName", "last_name")
	if err := setContactFields(body, inputs); err != nil {
		return nil, err
	}

	// UpsertRecord strips the match field from the payload for us — Salesforce
	// rejects a body that also sets the field named in the URL.
	id, created, raw, err := salesforce.UpsertRecord(instanceURL, token, "Contact", extField, extValue, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// An upsert that MATCHED an existing contact answers 204 No Content, so
	// Salesforce tells us nothing at all — not even which record it touched.
	// n8n stops there and hands the operator a bare success flag that nothing
	// downstream can chain off. One cheap SOQL read on the same match field
	// recovers the ID, which is the whole point of putting this action in a flow.
	if id == "" {
		id = lookupContactID(instanceURL, token, extField, extValue)
	}
	// Same reasoning for the body: a 204 leaves nothing to hand downstream, so
	// echo the ID and exactly the fields that were sent — those ARE the record's
	// new values, and re-reading the whole contact would cost another call
	// against the org's daily API allowance on every single upsert.
	if len(raw) == 0 {
		raw = map[string]interface{}{"Id": id, extField: extValue}
		for field, value := range body {
			raw[field] = value
		}
	}

	verb := "Updated"
	if created {
		verb = "Created"
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("%s contact %s matched on %s = %s", verb, displayName(inputs, extValue), extField, extValue)), nil
}

// displayName builds something readable for the summary line, falling back to
// the match value when the operator sent no name at all — which is the normal
// shape of an upsert that only touches one field.
func displayName(inputs []*core.Connection, fallback string) string {
	first := salesforce.OptionalString("first_name", inputs)
	last := salesforce.OptionalString("last_name", inputs)
	switch {
	case first != "" && last != "":
		return first + " " + last
	case last != "":
		return last
	case first != "":
		return first
	}
	return fallback
}

// lookupContactID re-reads the record ID after an upsert that returned no body.
//
// A failure here is deliberately swallowed: the upsert itself already succeeded,
// so turning a follow-up read problem into a flow failure would be a lie about
// what happened. The action simply returns without an ID in that case.
// The match value is operator-supplied and the match field is whichever field
// their administrator marked as an External ID — which is often a number. A
// number field rejects the quoted literal '1042' outright, so this reads the
// field's real type from one cached describe (BuildQueryTyped) and falls back to
// the untyped heuristic if describe is denied.
func lookupContactID(instanceURL, token, extField, extValue string) string {
	soql, err := salesforce.BuildQueryTyped(instanceURL, token, "Contact", "Id", []salesforce.Condition{
		{Field: extField, Operator: "=", Value: extValue},
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

// setContactFields maps the optional named inputs onto their Salesforce API
// names. See contact_create for the full rationale; the same function is
// restated in contact_create and contact_update, so keep the three in step.
//
// Last Name is handled by the caller here rather than in this list, because it
// is a required top-level input on create and an optional one on upsert and
// update.
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
