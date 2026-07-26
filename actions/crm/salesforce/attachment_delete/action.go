// Package crm_salesforce_attachment_delete removes a Classic Attachment.
//
// Salesforce sends it to the Recycle Bin rather than destroying it, so this is
// recoverable for 15 days — worth saying out loud, because "delete" reads as
// permanent to most people and the honest answer changes how carefully they
// build the flow around it.
package crm_salesforce_attachment_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Attachment (Classic)"
	Description  = "Delete a Classic attachment from the record it is on. It goes to the Recycle Bin, so it can be restored for 15 days."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "attachment_id", Type: core.ConnectionTypeString, Label: "Attachment", Placeholder: "Attachment ID, e.g. 00P5f00000XyzAAB", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment ID"},
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

	attachmentID, err := salesforce.RequiredString("attachment_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("attachment_id is required — the attachment to delete")
	}
	if err := salesforce.ValidateRecordID(attachmentID); err != nil {
		return nil, err
	}

	if err := salesforce.DeleteRecord(instanceURL, token, "Attachment", attachmentID); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers a delete with 204 No Content, so the ID we already
	// hold is the whole result — returning an empty map would break every
	// downstream step that wants to log or confirm what was removed.
	return salesforce.RecordResult(attachmentID, map[string]interface{}{"Id": attachmentID, "deleted": true},
		fmt.Sprintf("Deleted attachment %s — it is in the Recycle Bin for 15 days", attachmentID)), nil
}
