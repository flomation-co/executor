// Package crm_salesforce_attachment_get reads a Classic Attachment's record.
//
// The trap worth knowing: the Body field that comes back is a URL PATH, not the
// file. Salesforce returns "/services/data/v62.0/sobjects/Attachment/00P…/Body"
// there, and n8n surfaces it as-is, which is why people wire it into an email
// step and send a link nobody can open. This action returns the metadata and
// points at attachment_download for the actual bytes.
package crm_salesforce_attachment_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Attachment (Classic)"
	Description  = "Look up a Classic attachment's details — its name, size, type, owner and which record it belongs to. Use Download Attachment to get the file itself."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+eye"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "attachment_id", Type: core.ConnectionTypeString, Label: "Attachment", Placeholder: "Attachment ID, e.g. 00P5f00000XyzAAB", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields to Return", Placeholder: "Leave blank for everything, or e.g. Id,Name,ContentType,BodyLength"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Attachment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Attachment"},
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
		return nil, fmt.Errorf("attachment_id is required — the attachment you want to look up")
	}
	if err := salesforce.ValidateRecordID(attachmentID); err != nil {
		return nil, err
	}

	// A misspelled field name is rejected locally and never reaches Salesforce,
	// so it is a configuration mistake and takes the hard error return, matching
	// account_get and case_get. Left to GetRecord it would surface on the soft
	// error port as though Salesforce had refused a well-formed request.
	fields := salesforce.OptionalString("fields", inputs)
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, fmt.Errorf("Fields — %w", err)
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Attachment", attachmentID, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	name, _ := record["Name"].(string)
	if name == "" {
		name = attachmentID
	}
	return salesforce.RecordResult(attachmentID, record, fmt.Sprintf("Found attachment %s", name)), nil
}
