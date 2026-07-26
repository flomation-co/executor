package crm_salesforce_custom_object_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Custom Object Record"
	Description  = "Change fields on an existing record of one of your organisation's own Salesforce objects. Only the fields you fill in are sent, so everything else on the record is left exactly as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "custom_object", Type: core.ConnectionTypeString, Label: "Custom Object", Placeholder: "Invoice__c — the object's API name, which almost always ends in __c", Required: true},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record ID", Placeholder: "a015f00000ABCdeAAF — 15 or 18 characters, from the record's web address", Required: true},
	{Name: "record_name", Type: core.ConnectionTypeString, Label: "Record Name", Placeholder: "INV-1042 — leave blank to keep the current name"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "005... — the Salesforce user to hand the record to (leave blank to keep the current owner)"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "012... — only needed when moving the record to a different record type"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Fields (JSON)", Placeholder: `{"Status__c":"Paid","Paid_On__c":"2026-08-01"} — keyed by Salesforce field API name`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
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

	object, err := salesforce.RequiredString("custom_object", inputs)
	if err != nil {
		return nil, err
	}
	object, err = salesforce.ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}

	recordID, err := salesforce.RequiredString("record_id", inputs)
	if err != nil {
		return nil, err
	}
	if err := salesforce.ValidateRecordID(recordID); err != nil {
		return nil, err
	}

	// Owner and Record Type are Salesforce IDs, and a malformed one is the same
	// class of mistake as a typo'd record ID — a hard failure, not something the
	// flow's error branch has to interpret.
	for _, lookup := range []struct{ input, label string }{
		{"owner_id", "Owner"},
		{"record_type_id", "Record Type"},
	} {
		if v := salesforce.OptionalString(lookup.input, inputs); v != "" {
			if err := salesforce.ValidateRecordID(v); err != nil {
				return nil, fmt.Errorf("%s — %w", lookup.label, err)
			}
		}
	}

	// This is the update rule that bites hardest: an omitted field is left alone,
	// but a field sent as blank is CLEARED. Building the body from only the
	// inputs the operator actually filled in is what stops a partial update from
	// wiping half the record.
	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Name", "record_name")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field, or add the fields to change under Fields (JSON)")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, object, recordID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers a successful update with 204 No Content — no body, no
	// record. Echo the ID and the values just written rather than returning an
	// empty object, so the next node in the flow has something to chain off.
	// (An Apex trigger or workflow in the org may have adjusted these values
	// server-side; re-read the record if the flow depends on the stored state.)
	record := map[string]interface{}{"Id": recordID}
	for field, value := range body {
		record[field] = value
	}

	changed := salesforce.SortedKeys(body)
	summary := fmt.Sprintf("Updated %d field(s) on %s record %s: %s", len(changed), object, recordID, strings.Join(changed, ", "))
	return salesforce.RecordResult(recordID, record, summary), nil
}
