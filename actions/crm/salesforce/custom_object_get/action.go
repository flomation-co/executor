package crm_salesforce_custom_object_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Custom Object Record"
	Description  = "Read a single record from one of your organisation's own Salesforce objects using its record ID. Returns every field the connected Salesforce user can see, or just the ones you name."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "custom_object", Type: core.ConnectionTypeString, Label: "Custom Object", Placeholder: "Invoice__c — the object's API name, which almost always ends in __c", Required: true},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record ID", Placeholder: "a015f00000ABCdeAAF — 15 or 18 characters, from the record's web address", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. Name,Amount__c,Status__c (leave blank for all of them)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Record"},
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
	// A malformed ID is a configuration mistake, and catching it here turns
	// Salesforce's terse MALFORMED_ID into a message that says what an ID looks
	// like. The same check runs inside GetRecord; doing it first is what keeps
	// the failure off the error port.
	if err := salesforce.ValidateRecordID(recordID); err != nil {
		return nil, err
	}

	// Field names cannot be escaped in a Salesforce request, only whitelisted, so
	// validate the list up front — again so a typo is a hard failure rather than
	// something the flow's error branch has to interpret.
	fields := salesforce.OptionalString("fields", inputs)
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, err
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, object, recordID, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Custom objects carry a Name, so lead with it — "INV-1042" means something
	// to the person reading the run history and "a015f00000ABCdeAAF" does not.
	summary := fmt.Sprintf("Retrieved %s record %s", object, recordID)
	if name, ok := record["Name"].(string); ok && name != "" {
		summary = fmt.Sprintf("Retrieved %s record %q (%s)", object, name, recordID)
	}
	return salesforce.RecordResult(recordID, record, summary), nil
}
