// Package crm_salesforce_lead_delete removes a Lead.
//
// Salesforce sends a deleted record to the Recycle Bin rather than destroying
// it, so this is recoverable for 15 days — worth saying plainly, because a
// no-code operator who has just deleted the wrong lead needs to know they have
// not lost it.
package crm_salesforce_lead_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Lead"
	Description  = "Delete a lead. It goes to your Salesforce Recycle Bin and can be restored there for 15 days."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "lead_id", Type: core.ConnectionTypeString, Label: "Lead ID", Placeholder: "00Q5f000004XyzAEAS", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deleted Lead ID"},
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

	leadID, err := salesforce.RequiredString("lead_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("lead_id is required — the Salesforce record ID of the lead to delete, e.g. 00Q5f000004XyzAEAS")
	}
	// Checking the ID shape locally matters more on a delete than anywhere
	// else: a mistyped ID that happens to be valid deletes somebody else's
	// record, and one that is malformed should never reach Salesforce at all.
	if err := salesforce.ValidateRecordID(leadID); err != nil {
		return nil, err
	}

	if err := salesforce.DeleteRecord(instanceURL, token, "Lead", leadID); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// 204 No Content: nothing comes back, so return the ID that was deleted
	// along with the recoverable flag downstream branches can key off.
	deleted := map[string]interface{}{"Id": leadID, "deleted": true, "recoverable": true}
	return salesforce.RecordResult(leadID, deleted, fmt.Sprintf("Deleted lead %s — it is in the Recycle Bin and can be restored for 15 days", leadID)), nil
}
