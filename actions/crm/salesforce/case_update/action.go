// Package crm_salesforce_case_update changes fields on an existing support
// Case — reassign it, re-prioritise it, escalate it, or move it along the
// status pipeline.
//
// The one rule that governs this whole file: a field the operator left blank is
// NOT sent. Salesforce treats an omitted field (leave it alone) and an
// explicitly empty one (wipe it) as different instructions, so an update that
// posted every input would quietly clear the subject, description and account
// of any case it touched. Every mapping therefore goes through Set*IfPresent,
// and clearing a field deliberately is done through additional_fields with an
// explicit null.
package crm_salesforce_case_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Case"
	Description  = "Change an existing Salesforce case — reassign it, raise its priority, escalate it or move it to the next status. Fields you leave blank are left exactly as they are."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	{Name: "case_id", Type: core.ConnectionTypeString, Label: "Case ID", Placeholder: "5005f00000XyzAAAAQ — the case to change", Required: true},

	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Leave blank to keep the current subject"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Leave blank to keep the current description"},

	{Name: "case_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Working, Escalated or Closed (must match a status in your org)"},
	{Name: "priority", Type: core.ConnectionTypeString, Label: "Priority", Placeholder: "High, Medium or Low"},
	{Name: "case_type", Type: core.ConnectionTypeString, Label: "Case Type", Placeholder: "Problem, Question or Feature Request"},
	{Name: "case_origin", Type: core.ConnectionTypeString, Label: "How It Came In", Placeholder: "Phone, Email or Web"},
	{Name: "case_reason", Type: core.ConnectionTypeString, Label: "Reason", Placeholder: "Instructions not clear, Equipment complexity…"},

	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account", Placeholder: "Move the case to another account, e.g. 0015f00000XyzAAAAQ"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact", Placeholder: "0035f00000XyzAAAAQ"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Reassign To", Placeholder: "Salesforce user ID (005…) or support queue ID (00G…)"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Parent Case", Placeholder: "Group this under an existing case, e.g. 5005f00000XyzAAAAQ"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Only if your org uses record types, e.g. 0125f000000XyzAAA"},

	{Name: "is_escalated", Type: core.ConnectionTypeBoolean, Label: "Escalated"},

	{Name: "supplied_name", Type: core.ConnectionTypeString, Label: "Reported By (Name)", Placeholder: "Jane Smith"},
	{Name: "supplied_email", Type: core.ConnectionTypeString, Label: "Reported By (Email)", Placeholder: "jane.smith@acme.com"},
	{Name: "supplied_phone", Type: core.ConnectionTypeString, Label: "Reported By (Phone)", Placeholder: "+44 20 7946 0958"},
	{Name: "supplied_company", Type: core.ConnectionTypeString, Label: "Reported By (Company)", Placeholder: "Acme Ltd"},

	// Also the only way to CLEAR a field: pass {"Description": null}. A blank
	// named input above means "leave it alone", so an explicit null here is the
	// deliberate, unambiguous way to empty something.
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"SLA_Tier__c":"Gold"} — or {"Description":null} to clear a field`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Case ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Fields Written"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	caseID, err := salesforce.RequiredString("case_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("case_id is required — the ID of the case to update")
	}
	if err := salesforce.ValidateRecordID(caseID); err != nil {
		return nil, err
	}

	body := map[string]interface{}{}
	if err := setCaseFields(body, inputs); err != nil {
		return nil, err
	}
	// n8n sends an empty PATCH here and Salesforce accepts it as a no-op, which
	// looks like a success and changes nothing. An update with nothing to update
	// is a configuration mistake, so say so instead.
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field, or supply additional_fields")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Case", caseID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers a successful PATCH with 204 No Content: there is no
	// record to return, only the ID we already had and the fields we wrote. The
	// ID matters — without it nothing downstream can chain off the update.
	//
	// SortedKeys, not map order, so the same update reads identically in the run
	// log on every run.
	written := salesforce.SortedKeys(body)
	result := map[string]interface{}{"Id": caseID, "updated_fields": written}
	return salesforce.RecordResult(caseID, result, fmt.Sprintf("Updated case %s — %d field(s) changed: %s", caseID, len(written), strings.Join(written, ", "))), nil
}

// setCaseFields maps the named inputs onto their Salesforce API names.
//
// The mapping is spelled out rather than derived from the input name because it
// is not mechanical: owner_id becomes OwnerId, but case_type becomes plain Type
// and case_reason becomes plain Reason, with no "Case" prefix at all.
//
// Note what is NOT here: nothing writes an empty string. A blank input is
// omitted entirely, which is the whole reason an update can be run safely from
// a form where the operator only filled in one box.
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

	// SetBoolIfSet, not a truthiness test: an explicit "false" is transmitted,
	// which is the only way to de-escalate a case that was escalated.
	salesforce.SetBoolIfSet(body, inputs, "IsEscalated", "is_escalated")

	// Additional fields go on last so a custom value deliberately wins over a
	// named input set to the same API name.
	return salesforce.MergeAdditionalFields(body, inputs)
}
