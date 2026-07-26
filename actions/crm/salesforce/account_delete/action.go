package crm_salesforce_account_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Account"
	Description  = "Send a Salesforce account to the Recycle Bin, where it can be restored for 15 days. Contacts, opportunities and cases under the account go with it."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account", Placeholder: "Record ID of the account to delete, e.g. 0015f00000AbCdEAAV", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Account ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deleted Account"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	accountID := salesforce.OptionalString("account_id", inputs)
	if accountID == "" {
		return nil, fmt.Errorf("account_id is required — the record ID of the account to delete, e.g. 0015f00000AbCdEAAV")
	}
	if err := salesforce.ValidateRecordID(accountID); err != nil {
		return nil, err
	}

	if err := salesforce.DeleteRecord(instanceURL, token, "Account", accountID); err != nil {
		// Deleting a record that is already gone answers ENTITY_IS_DELETED,
		// which CheckResponse translates. It is a provider outcome, not a
		// configuration mistake, so it takes the error port as data.
		return salesforce.ErrorResult(err.Error()), nil
	}

	// A successful DELETE is 204 No Content, so the ID the operator supplied is
	// the only thing there is to return — and it is what a downstream node
	// needs in order to log or undo the deletion.
	return salesforce.RecordResult(accountID, map[string]interface{}{"Id": accountID, "deleted": true}, fmt.Sprintf("Sent account %s to the Recycle Bin", accountID)), nil
}
