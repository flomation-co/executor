package crm_salesforce_account_contact_relation_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Remove Contact from Account"
	Description  = "Remove the link between a contact and one of the other companies they work with. The contact and both companies are untouched — only the relationship is removed."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-minus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "relation_id", Type: core.ConnectionTypeString, Label: "Relationship", Placeholder: "Relationship ID from Get Many Account-Contact Relationships, e.g. 07n5f000000AbCdAAK", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Relationship ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Removed Relationship"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	relationID := salesforce.OptionalString("relation_id", inputs)
	if relationID == "" {
		return nil, fmt.Errorf("relation_id is required — the relationship's own record ID, which Get Many Account-Contact Relationships returns, e.g. 07n5f000000AbCdAAK")
	}
	if err := salesforce.ValidateRecordID(relationID); err != nil {
		return nil, err
	}

	if err := salesforce.DeleteRecord(instanceURL, token, "AccountContactRelation", relationID); err != nil {
		// The relationship to a contact's OWN account is maintained by
		// Salesforce (IsDirect = true) and cannot be deleted through the API —
		// move the contact to a different account instead. Salesforce reports
		// that as an access error, which CheckResponse explains.
		return salesforce.ErrorResult(err.Error()), nil
	}

	// DELETE answers 204 No Content, so the ID the operator supplied is all
	// there is to hand back — and it is what a downstream node needs to log
	// what was removed.
	return salesforce.RecordResult(relationID, map[string]interface{}{"Id": relationID, "deleted": true}, fmt.Sprintf("Removed account-contact relationship %s", relationID)), nil
}
