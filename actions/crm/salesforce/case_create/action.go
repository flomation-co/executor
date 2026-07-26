// Package crm_salesforce_case_create raises a support Case — the record a
// Salesforce org uses for a customer problem, question or complaint. It is the
// obvious landing point for every inbound helpdesk automation a front-of-house
// team builds ("support inbox email → open a case and tell the team channel"),
// so the whole standard field set is exposed as named inputs rather than making
// the operator hand-assemble JSON.
//
// Two deliberate departures from n8n's node, both in the operator's favour:
// Case Type is optional here (n8n makes it mandatory, which is an n8n choice —
// Salesforce itself requires no field on Case at all), and Parent Case actually
// works (n8n declares the field as "ParentId" but reads "parentId", so it is
// silently never sent and case hierarchies cannot be built through its UI).
package crm_salesforce_case_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Case"
	Description  = "Raise a support case in Salesforce from an email, phone call or web form, and link it to the customer's account or contact. Every field is optional — Salesforce accepts a case with nothing but a subject."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Printer not working after the office move"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "The full detail of the problem, in the customer's own words"},

	// Status, Priority, Type, Origin and Reason are all picklists, and every org
	// edits its own lists. They are plain text here: the value has to match one
	// of your org's options exactly or Salesforce rejects the whole case.
	{Name: "case_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "New, Working or Escalated (must match a status in your org)"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "Priority", Placeholder: "High, Medium or Low"},
	{Name: "case_type", Type: core.ConnectionTypeString, Label: "Case Type", Placeholder: "Problem, Question or Feature Request"},
	{Name: "case_origin", Type: core.ConnectionTypeString, Label: "How It Came In", Placeholder: "Phone, Email or Web"},
	{Name: "case_reason", Type: core.ConnectionTypeString, Label: "Reason", Placeholder: "Instructions not clear, Equipment complexity…"},

	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account", Placeholder: "The customer's account ID, e.g. 0015f00000XyzAAAAQ"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact", Placeholder: "The person who reported it, e.g. 0035f00000XyzAAAAQ"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Assign To", Placeholder: "Salesforce user ID (005…) or support queue ID (00G…)"},
	// Parent Case is the field n8n advertises but never sends. Setting it is how
	// duplicate reports of one outage get grouped under a single master case.
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Parent Case", Placeholder: "Group this under an existing case, e.g. 5005f00000XyzAAAAQ"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Only if your org uses record types, e.g. 0125f000000XyzAAA"},

	{Name: "is_escalated", Type: core.ConnectionTypeBoolean, Label: "Escalated"},

	// The Supplied* fields are where web-to-case puts the details of someone who
	// is not yet in the CRM. They are the right place for an enquiry from an
	// unknown sender, because they do not require a Contact to exist first.
	{Name: "supplied_name", Type: core.ConnectionTypeString, Label: "Reported By (Name)", Placeholder: "Jane Smith — for someone not yet in Salesforce"},
	{Name: "supplied_email", Type: core.ConnectionTypeString, Label: "Reported By (Email)", Placeholder: "jane.smith@acme.com"},
	{Name: "supplied_phone", Type: core.ConnectionTypeString, Label: "Reported By (Phone)", Placeholder: "+44 20 7946 0958"},
	{Name: "supplied_company", Type: core.ConnectionTypeString, Label: "Reported By (Company)", Placeholder: "Acme Ltd"},

	// Every Salesforce org has custom fields, so the escape hatch is the normal
	// path here, not an edge case. Keys are Salesforce API names.
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Custom_Field__c":"value","SLA_Tier__c":"Gold"}`},

	// Off by default: it costs a second API call against the org's daily
	// allowance. Worth turning on whenever the flow goes on to tell someone
	// their case number — see the comment on fetchSavedCase.
	{Name: "return_record", Type: core.ConnectionTypeBoolean, Label: "Fetch the Saved Case (adds the Case Number)"},
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

	body := map[string]interface{}{}
	if err := setCaseFields(body, inputs); err != nil {
		// Malformed additional_fields JSON is a configuration mistake, so it
		// fails hard rather than landing on the error port as data.
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to create — fill in at least a subject, or supply additional_fields")
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Case", body)
	if err != nil {
		// Anything Salesforce rejected (a validation rule, a required custom
		// field, a picklist value the org does not have) is a provider failure,
		// so it lands on the error port as data rather than killing the flow.
		return salesforce.ErrorResult(err.Error()), nil
	}

	record := raw
	caseNumber := ""
	if salesforce.OptionalBool("return_record", inputs) {
		if saved := fetchSavedCase(instanceURL, token, id); saved != nil {
			record = saved
			caseNumber, _ = saved["CaseNumber"].(string)
		}
	}

	return salesforce.RecordResult(id, record, createSummary(id, caseNumber, salesforce.OptionalString("subject", inputs))), nil
}

// fetchSavedCase re-reads the case Salesforce just saved.
//
// The create response carries the record ID and nothing else, but the number a
// customer is actually quoted on the phone — CaseNumber — is generated by
// Salesforce at save time and appears only on the stored record. A flow that
// emails "we have logged your enquiry as case 00001026" cannot be built from
// the create response alone, which is why this exists.
//
// It is best effort: the case IS created by this point, so a failed read must
// not turn a successful write into an error. Log and fall back to the create
// envelope instead.
func fetchSavedCase(instanceURL, token, id string) map[string]interface{} {
	record, err := salesforce.GetRecord(instanceURL, token, "Case", id, "")
	if err != nil {
		log.WithError(err).WithField("case_id", id).Warn("Salesforce create case: the case was created but could not be read back; returning the create response instead")
		return nil
	}
	return record
}

// createSummary builds the one-line result an operator reads in the run log,
// leading with the case number when we have it because that is the reference
// they will be asked for, not the 18-character record ID.
func createSummary(id, caseNumber, subject string) string {
	label := "case " + id
	if caseNumber != "" {
		label = fmt.Sprintf("case %s (%s)", caseNumber, id)
	}
	if subject != "" {
		return fmt.Sprintf("Created %s — %s", label, subject)
	}
	return fmt.Sprintf("Created %s", label)
}

// setCaseFields maps the named inputs onto their Salesforce API names.
//
// The mapping is spelled out rather than derived from the input name because it
// is not mechanical: owner_id becomes OwnerId, but case_type becomes plain Type
// and case_reason becomes plain Reason, with no "Case" prefix at all. A generic
// "capitalise and strip underscores" transform would produce CaseType, which
// does not exist, and the error would only surface at run time.
//
// Every write goes through Set*IfPresent so an input the operator left blank is
// OMITTED from the payload rather than sent as an empty string. On create that
// only decides which defaults Salesforce applies; keeping create and update on
// the identical mapping is what stops the two drifting apart, and on update the
// difference is destructive.
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

	// SetBoolIfSet, not a truthiness test, so an explicit "false" is still
	// transmitted — otherwise there would be no way to create a case that is
	// deliberately flagged not-escalated in an org whose default is true.
	salesforce.SetBoolIfSet(body, inputs, "IsEscalated", "is_escalated")

	// Additional fields go on last so a custom value deliberately wins over a
	// named input set to the same API name.
	return salesforce.MergeAdditionalFields(body, inputs)
}
