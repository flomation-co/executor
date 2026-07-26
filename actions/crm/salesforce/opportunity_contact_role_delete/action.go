package crm_salesforce_opportunity_contact_role_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Remove Contact from Opportunity"
	Description  = "Unlink a contact from a deal. The contact record itself is untouched - only their involvement in this deal is removed."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-minus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "contact_role_id", Type: core.ConnectionTypeString, Label: "Contact Role ID", Placeholder: "00K5f00000AbCdEAAV - from Get Opportunity Contacts", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact Role ID"},
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

	id := salesforce.OptionalString("contact_role_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// This deletes the junction record, not the Contact. Worth stating plainly
	// because "remove the contact" reads alarmingly to the person clicking it.
	// A second run on the same ID answers ENTITY_IS_DELETED, which is Salesforce
	// telling us something rather than a wiring mistake — so it goes out on the
	// error port as data.
	if err := salesforce.DeleteRecord(instanceURL, token, "OpportunityContactRole", id); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers 204 No Content, so the ID we were given is all there is
	// to hand back.
	record := map[string]interface{}{"Id": id, "deleted": true}
	return salesforce.RecordResult(id, record, fmt.Sprintf("Removed contact role %s from its opportunity", id)), nil
}
