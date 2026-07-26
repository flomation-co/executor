package crm_salesforce_custom_object_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Custom Object Record"
	Description  = "Send a record of one of your organisation's own Salesforce objects to the Recycle Bin. It stays recoverable there for 15 days, so this is not immediately permanent."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "custom_object", Type: core.ConnectionTypeString, Label: "Custom Object", Placeholder: "Invoice__c — the object's API name, which almost always ends in __c", Required: true},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record ID", Placeholder: "a015f00000ABCdeAAF — 15 or 18 characters, from the record's web address", Required: true},
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

	recordID, err := salesforce.RequiredString("record_id", inputs)
	if err != nil {
		return nil, err
	}
	if err := salesforce.ValidateRecordID(recordID); err != nil {
		return nil, err
	}

	if err := salesforce.DeleteRecord(instanceURL, token, object, recordID); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// A successful delete is 204 No Content, so the ID we were given is the only
	// thing there is to return — and it is the thing a later "restore" step or an
	// audit log actually needs.
	record := map[string]interface{}{"Id": recordID, "deleted": true}
	summary := fmt.Sprintf("Deleted %s record %s — it is in the Salesforce Recycle Bin and can be restored for 15 days", object, recordID)
	return salesforce.RecordResult(recordID, record, summary), nil
}
