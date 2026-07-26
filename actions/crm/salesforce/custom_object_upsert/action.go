package crm_salesforce_custom_object_upsert

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create or Update Custom Object Record"
	Description  = "Find a record on one of your organisation's own Salesforce objects by an External ID field and update it, or create it if nothing matches. This is how you keep Salesforce in step with another system without ever creating duplicates."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+arrow-right-arrow-left"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// The match field must be one Salesforce itself can look a record up by — a
// field marked External ID (or Id). An ordinary text field will be rejected with
// "The requested resource does not exist", which is why the placeholder spells
// out where the setting lives.
var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "custom_object", Type: core.ConnectionTypeString, Label: "Custom Object", Placeholder: "Invoice__c — the object's API name, which almost always ends in __c", Required: true},
	{Name: "external_id_field", Type: core.ConnectionTypeString, Label: "Match On Field", Placeholder: "Invoice_Number__c — must be ticked as an External ID on the field in Salesforce Setup", Required: true},
	{Name: "external_id_value", Type: core.ConnectionTypeString, Label: "Match On Value", Placeholder: "INV-1042 — the value to look for in that field", Required: true},
	{Name: "record_name", Type: core.ConnectionTypeString, Label: "Record Name", Placeholder: "INV-1042 — leave blank if this object numbers its records automatically"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "005... — the Salesforce user who should own the record"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "012... — only needed when this object uses record types"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Fields (JSON)", Placeholder: `{"Amount__c":250,"Status__c":"Sent"} — keyed by Salesforce field API name`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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

	externalIDField, err := salesforce.RequiredString("external_id_field", inputs)
	if err != nil {
		return nil, err
	}
	externalIDField, err = salesforce.ValidateSOQLFieldName(externalIDField)
	if err != nil {
		return nil, err
	}
	externalIDValue, err := salesforce.RequiredString("external_id_value", inputs)
	if err != nil {
		return nil, err
	}

	// Owner and Record Type are Salesforce IDs, and a malformed one is a
	// configuration mistake, so fail hard here rather than letting Salesforce
	// answer MALFORMED_ID on the error port next to genuine provider failures.
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

	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Name", "record_name")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	// Unlike create, an upsert with no other fields is legitimate — it is how you
	// register that a record with this external ID exists — so an empty body is
	// not an error here. UpsertRecord strips the match field from the body for
	// us; Salesforce rejects a payload that sets the field it is matching on.

	id, created, raw, err := salesforce.UpsertRecord(instanceURL, token, object, externalIDField, externalIDValue, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// The two outcomes of the SAME call answer differently: a create comes back
	// as 201 with the new ID, a match comes back as 204 No Content with nothing
	// at all. Look the ID up so the routine case — a plain update — still hands
	// the next node something to work with.
	if id == "" {
		matched, lookupErr := resolveMatchedID(instanceURL, token, object, externalIDField, externalIDValue)
		switch {
		case lookupErr != nil:
			// The upsert itself succeeded, so this must never fail the action. A
			// connected user who can write but not read the object defeats the
			// lookup — record it and carry on.
			log.WithError(lookupErr).WithFields(log.Fields{
				"object":            object,
				"external_id_field": externalIDField,
			}).Warn("Salesforce upsert: could not resolve the ID of the record that was matched")
		case matched == "":
			// The lookup ran and matched nothing, which in practice means the
			// connected user cannot read back what it has just written. Say so,
			// rather than handing the flow a blank ID with nothing in the log to
			// explain it.
			log.WithFields(log.Fields{
				"object":            object,
				"external_id_field": externalIDField,
			}).Warn("Salesforce upsert: the record was written but could not be read back by its external ID, so no record ID is available")
		default:
			id = matched
		}
	}

	// 204 leaves nothing to return, so echo the ID and the values just written
	// rather than an empty object.
	record := raw
	if len(record) == 0 {
		record = map[string]interface{}{externalIDField: externalIDValue}
		for field, value := range body {
			record[field] = value
		}
	}
	// A create answers 201 with Salesforce's own {id, success, created} envelope
	// and a match answers 204 with nothing at all, so without this the two halves
	// of the SAME action hand the next node different shapes — Id present on an
	// update, absent on a create. Both branches get Id and created, and created is
	// what a flow tests to decide whether this record is new.
	if id != "" {
		record["Id"] = id
	}
	record["created"] = created

	var summary string
	switch {
	case created:
		summary = fmt.Sprintf("Created %s record %s — nothing matched %s = %s", object, id, externalIDField, externalIDValue)
	case id != "":
		summary = fmt.Sprintf("Updated the existing %s record %s, matched on %s = %s", object, id, externalIDField, externalIDValue)
	default:
		summary = fmt.Sprintf("Updated the existing %s record matched on %s = %s — Salesforce did not return its ID", object, externalIDField, externalIDValue)
	}
	return salesforce.RecordResult(id, record, summary), nil
}

// resolveMatchedID finds the ID of the record an upsert has just matched.
//
// Salesforce answers an upsert that CREATED a record with 201 and a body
// carrying the new ID, but an upsert that MATCHED an existing record with 204 No
// Content — no body, no ID. That means the most common outcome of this action (a
// routine update of a record that already exists) would otherwise return a blank
// ID and nothing downstream could chain off it. One extra SOQL call on that path
// is a fair price, and it is the same lookup Salesforce just did internally.
//
// The match value is operator-supplied and the match field is whichever field
// the org marked as an External ID — very often an auto-number or number field,
// where the quoted literal '1042' is an outright INVALID_FIELD. BuildQueryTyped
// reads the field's real type from one cached describe so the literal is
// rendered the way the field demands, and degrades to the untyped heuristic when
// the connected user cannot describe the object.
func resolveMatchedID(instanceURL, token, object, externalIDField, externalIDValue string) (string, error) {
	soql, err := salesforce.BuildQueryTyped(instanceURL, token, object, "Id", []salesforce.Condition{{
		Field:    externalIDField,
		Operator: "=",
		Value:    externalIDValue,
	}}, false, "", 1, true)
	if err != nil {
		return "", err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", nil
	}
	return salesforce.StringifyID(record["Id"]), nil
}
