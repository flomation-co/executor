// Package crm_salesforce_contact_delete removes a Contact from Salesforce.
//
// Salesforce deletes are soft: the record goes to the org's Recycle Bin and can
// be restored for 15 days, which is worth saying out loud in the action's
// summary because "delete" reads as permanent to most operators.
package crm_salesforce_contact_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Contact"
	Description  = "Send a contact to the Salesforce Recycle Bin, where an administrator can restore it for the next 15 days."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "0035f00000XyzAbAAJ — from the contact's Salesforce URL", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
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

	contactID, err := salesforce.RequiredString("contact_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("contact_id is required — the 15 or 18 character ID from the contact's Salesforce URL")
	}
	if err := salesforce.ValidateRecordID(contactID); err != nil {
		return nil, err
	}

	if err := salesforce.DeleteRecord(instanceURL, token, "Contact", contactID); err != nil {
		// Already deleted, locked by a trigger, or not visible to this user —
		// all provider answers, so they land on the error port as data.
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers a delete with 204 No Content, so the ID the operator
	// supplied is the only thing there is to hand downstream — and it is exactly
	// what a follow-up "log what we removed" step needs.
	return salesforce.RecordResult(contactID, map[string]interface{}{"Id": contactID, "deleted": true},
		fmt.Sprintf("Deleted contact %s — it is in the Recycle Bin for 15 days", contactID)), nil
}
