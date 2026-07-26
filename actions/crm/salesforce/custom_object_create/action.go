package crm_salesforce_custom_object_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Custom Object Record"
	Description  = "Add a record to one of your organisation's own Salesforce objects — an Invoice, a Booking, a Property, whatever your administrator has built. Fill in the standard Name, Owner and Record Type, and supply the object's own fields as JSON."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// Every Salesforce object — standard or custom — is created through the same
// POST /sobjects/{type} endpoint, which is why this action needs no per-object
// knowledge at all. What it cannot know is the COLUMNS: those are defined by the
// customer's administrator, differ in every org, and change without notice. So
// the three fields every custom object always has (Name, Owner, Record Type)
// are first-class inputs and everything else arrives as a JSON object.
var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "custom_object", Type: core.ConnectionTypeString, Label: "Custom Object", Placeholder: "Invoice__c — the object's API name, which almost always ends in __c", Required: true},
	{Name: "record_name", Type: core.ConnectionTypeString, Label: "Record Name", Placeholder: "INV-1042 — leave blank if this object numbers its records automatically"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "005... — the Salesforce user who should own the record (defaults to the connected user)"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "012... — only needed when this object uses record types"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Fields (JSON)", Placeholder: `{"Amount__c":250,"Due_Date__c":"2026-08-01","Status__c":"Draft"} — keyed by Salesforce field API name`},
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
	// Validate the object name here rather than leaving it to CreateRecord, so a
	// typo ("Invoice c") fails hard as the configuration mistake it is instead of
	// landing on the error port next to genuine Salesforce failures.
	object, err = salesforce.ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}

	// Owner and Record Type are Salesforce IDs, and a malformed one is the same
	// class of mistake as a typo'd object name. Left to Salesforce it comes back
	// as MALFORMED_ID on the error port, sat next to genuine provider failures.
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

	// Only fields the operator actually filled in are sent. Salesforce treats an
	// omitted field and an explicit null differently, and on a custom object with
	// required fields a stray null is rejected outright rather than ignored.
	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Name", "record_name")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		// Salesforce answers a bodyless create with a bare 400 that names nothing,
		// so catch it here where we can say what to do about it.
		return nil, fmt.Errorf("no field values were set — give the record a name, or add at least one field under Fields (JSON)")
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, object, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Created %s record %s", object, id)), nil
}
