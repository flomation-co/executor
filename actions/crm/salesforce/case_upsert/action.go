// Package crm_salesforce_case_upsert creates a Case, or updates the existing
// one that already carries the same external ID.
//
// This is the idempotency primitive for helpdesk sync, and n8n has no upsert on
// Case at all — only on Lead, Contact, Account, Opportunity and custom objects.
// The consequence is concrete: a webhook that retries, or a flow re-run after a
// failure, opens a second copy of every case. Matching on the ticket reference
// from the system of record ("Zendesk_Ticket__c") makes the second run update
// the case the first run created, however many times it fires.
//
// The external ID field must be marked External ID (or Unique + idLookup) in
// Salesforce Setup. Any other field is rejected — that is Salesforce's rule, not
// ours, and the error explains it because it is the one thing that goes wrong
// when an operator first sets this up.
package crm_salesforce_case_upsert

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create or Update Case"
	Description  = "Create a Salesforce case, or update the existing one with the same reference. Use it to sync tickets from another system without ever creating a duplicate, however many times the flow re-runs."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+arrow-right-arrow-left"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	{Name: "external_id_field", Type: core.ConnectionTypeString, Label: "Match On Field", Placeholder: "Zendesk_Ticket__c — must be marked External ID in Salesforce", Required: true},
	{Name: "external_id_value", Type: core.ConnectionTypeString, Label: "Match On Value", Placeholder: "The reference from the other system, e.g. ZD-10482", Required: true},

	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Printer not working after the office move"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "The full detail of the problem, in the customer's own words"},

	{Name: "case_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "New, Working or Escalated (must match a status in your org)"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "Priority", Placeholder: "High, Medium or Low"},
	{Name: "case_type", Type: core.ConnectionTypeString, Label: "Case Type", Placeholder: "Problem, Question or Feature Request — must match your org's Case Type list"},
	{Name: "case_origin", Type: core.ConnectionTypeString, Label: "How It Came In", Placeholder: "Phone, Email or Web"},
	{Name: "case_reason", Type: core.ConnectionTypeString, Label: "Reason", Placeholder: "Instructions not clear, Equipment complexity — must match your org's Case Reason list"},

	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account", Placeholder: "The customer's account ID, e.g. 0015f00000XyzAAAAQ"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact", Placeholder: "The person who reported it, e.g. 0035f00000XyzAAAAQ"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Assign To", Placeholder: "Salesforce user ID (005…) or support queue ID (00G…)"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Parent Case", Placeholder: "Group this under an existing case, e.g. 5005f00000XyzAAAAQ"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Only if your org uses record types, e.g. 0125f000000XyzAAA"},

	{Name: "is_escalated", Type: core.ConnectionTypeBoolean, Label: "Escalated"},

	{Name: "supplied_name", Type: core.ConnectionTypeString, Label: "Reported By (Name)", Placeholder: "Jane Smith — for someone not yet in Salesforce"},
	{Name: "supplied_email", Type: core.ConnectionTypeString, Label: "Reported By (Email)", Placeholder: "jane.smith@acme.com"},
	{Name: "supplied_phone", Type: core.ConnectionTypeString, Label: "Reported By (Phone)", Placeholder: "+44 20 7946 0958"},
	{Name: "supplied_company", Type: core.ConnectionTypeString, Label: "Reported By (Company)", Placeholder: "Acme Ltd"},

	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Custom_Field__c":"value","SLA_Tier__c":"Gold"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Case ID"},
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
		return nil, fmt.Errorf("external_id_field is required — the Salesforce field holding the reference from your other system, e.g. Zendesk_Ticket__c")
	}
	// Validate the field name here rather than inside UpsertRecord so a typo is
	// classified as what it is: a configuration mistake, not a Salesforce
	// failure to be retried.
	if externalField, err = salesforce.ValidateSOQLFieldName(externalField); err != nil {
		return nil, err
	}
	externalValue, err := salesforce.RequiredString("external_id_value", inputs)
	if err != nil {
		return nil, fmt.Errorf("external_id_value is required — it is the value Salesforce matches on, so an empty one would match nothing")
	}

	body := map[string]interface{}{}
	if err := setCaseFields(body, inputs); err != nil {
		return nil, err
	}

	recordID, created, raw, err := salesforce.UpsertRecord(instanceURL, token, "Case", externalField, externalValue, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	if recordID == "" {
		// Salesforce answers an upsert that MATCHED an existing case with 204 No
		// Content: no body, and therefore no record ID. Handing the flow an empty
		// ID would defeat the point of an idempotent write — nothing downstream
		// could comment on, close or link the case that was just updated — so
		// resolve it with the one lookup that is always available: the external
		// ID we just matched on.
		recordID = lookupByExternalID(instanceURL, token, externalField, externalValue)
	}

	verb := "Updated"
	if created {
		verb = "Created"
	}
	summary := fmt.Sprintf("%s case %s (matched on %s = %s)", verb, recordID, externalField, externalValue)
	if recordID == "" {
		summary = fmt.Sprintf("%s the case matching %s = %s, but Salesforce did not return its ID", verb, externalField, externalValue)
	}
	return salesforce.RecordResult(recordID, raw, summary), nil
}

// lookupByExternalID finds the record ID of the case an upsert just matched.
//
// Best effort by design: the write has already succeeded at this point, so a
// failed lookup must not turn it into an error — that would push a completed
// write onto the error port and invite a pointless retry. It logs instead and
// the summary says the ID is missing.
//
// The query is built through BuildQueryTyped so the operator-supplied field name
// and value go through the same validation and escaping as every other SOQL
// string in this node — and so the value is quoted according to what the
// external ID field actually IS. A numeric external ID needs a bare literal and
// a text one needs quotes; guessing from the value alone gets a reference like
// "10482" wrong every time.
func lookupByExternalID(instanceURL, token, externalField, externalValue string) string {
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"Case",
		"Id,CaseNumber",
		[]salesforce.Condition{{Field: externalField, Operator: "=", Value: externalValue}},
		false,
		"",
		1,
		true,
	)
	if err != nil {
		log.WithError(err).Warn("Salesforce upsert case: could not build the lookup for the matched case ID")
		return ""
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		// Rare now the value is rendered from the field's real type, but still
		// possible when describe is denied and the heuristic has to guess. The
		// upsert itself still worked.
		log.WithError(err).WithField("external_id_field", externalField).Warn("Salesforce upsert case: the case was written but its ID could not be looked up")
		return ""
	}
	if record == nil {
		return ""
	}
	return salesforce.StringifyID(record["Id"])
}

// setCaseFields maps the named inputs onto their Salesforce API names.
//
// The mapping is spelled out rather than derived from the input name because it
// is not mechanical: owner_id becomes OwnerId, but case_type becomes plain Type
// and case_reason becomes plain Reason, with no "Case" prefix at all.
//
// Set*IfPresent throughout, so a blank input is omitted. That matters more here
// than on a create: an upsert that matches an existing case is an UPDATE, and a
// payload carrying every empty box would wipe the subject and description of a
// case someone is working on.
//
// Note the match field itself is not set here — UpsertRecord strips it from the
// body, because Salesforce rejects a payload that also sets the field it is
// matching on.
func setCaseFields(body map[string]interface{}, inputs []*core.Connection) error {
	salesforce.SetIfPresent(body, inputs, "Subject", "subject")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "Status", "case_status")
	salesforce.SetIfPresent(body, inputs, "Priority", "priority")
	salesforce.SetIfPresent(body, inputs, "Type", "case_type")
	salesforce.SetIfPresent(body, inputs, "Origin", "case_origin")
	salesforce.SetIfPresent(body, inputs, "Reason", "case_reason")
	salesforce.SetIfPresent(body, inputs, "AccountId", "account_id")
	salesforce.SetIfPresent(body, inputs, "ContactId", "contact_id")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "ParentId", "parent_id")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")
	salesforce.SetIfPresent(body, inputs, "SuppliedName", "supplied_name")
	salesforce.SetIfPresent(body, inputs, "SuppliedEmail", "supplied_email")
	salesforce.SetIfPresent(body, inputs, "SuppliedPhone", "supplied_phone")
	salesforce.SetIfPresent(body, inputs, "SuppliedCompany", "supplied_company")

	salesforce.SetBoolIfSet(body, inputs, "IsEscalated", "is_escalated")

	// Additional fields go on last so a custom value deliberately wins over a
	// named input set to the same API name.
	return salesforce.MergeAdditionalFields(body, inputs)
}
