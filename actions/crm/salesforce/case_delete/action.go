// Package crm_salesforce_case_delete deletes a support Case.
//
// Salesforce soft-deletes: the case goes to the Recycle Bin and stays there for
// 15 days, so this is recoverable rather than final. The action says so in its
// description because "delete" reads as permanent to everyone who has not used
// Salesforce, and the difference decides whether an operator is willing to put
// it in a flow at all.
package crm_salesforce_case_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Case"
	Description  = "Send a Salesforce case to the Recycle Bin. It is recoverable there for 15 days, so this is not permanent — but the case disappears from list views and reports straight away."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "case_id", Type: core.ConnectionTypeString, Label: "Case ID", Placeholder: "5005f00000XyzAAAAQ — the case to delete", Required: true},
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

	caseID, err := salesforce.RequiredString("case_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("case_id is required — the ID of the case to delete")
	}
	// A malformed ID is a configuration mistake, so it fails hard here rather
	// than travelling to Salesforce and coming back as MALFORMED_ID.
	if err := salesforce.ValidateRecordID(caseID); err != nil {
		return nil, err
	}

	if err := salesforce.DeleteRecord(instanceURL, token, "Case", caseID); err != nil {
		// A case that was already deleted, or one the connected user cannot
		// delete, is Salesforce's answer — data on the error port, not a
		// flow-stopping failure.
		return salesforce.ErrorResult(err.Error()), nil
	}

	// DELETE answers 204 No Content, so the ID we were given is the only thing
	// there is to hand on. Returning it (rather than an empty result) is what
	// lets a later node log or announce which case went.
	result := map[string]interface{}{"Id": caseID, "deleted": true}
	return salesforce.RecordResult(caseID, result, fmt.Sprintf("Deleted case %s — it is in the Recycle Bin for 15 days", caseID)), nil
}
