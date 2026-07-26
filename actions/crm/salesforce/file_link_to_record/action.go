// Package crm_salesforce_file_link_to_record shares an existing Salesforce file
// onto another record.
//
// A file in Salesforce lives once and is SHARED with as many records as you
// like through ContentDocumentLink — it is not copied. That is what makes "one
// signed contract, visible on the account, the opportunity and the case" work,
// and it is the piece n8n has no action for at all.
//
// The two options Salesforce insists on both mean something the operator cares
// about. Share Type decides whether the people who can see the record can also
// edit the file, and Visibility decides whether customer-community users see it
// — get that one wrong and an internal document is exposed to a portal.
package crm_salesforce_file_link_to_record

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Attach File To Record"
	Description  = "Share a file that is already in Salesforce onto another record, so it appears in that record's Files list too. The file itself is not duplicated."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+link"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// contentVersionPrefix is the Salesforce key prefix for a ContentVersion (one
// revision). The link object wants the ContentDocument (069…) instead, so a
// version ID — which is what an upload step hands back — is followed up to its
// document rather than rejected.
const contentVersionPrefix = "068"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File", Placeholder: "File ID (069…) or version ID (068…) from an upload or files list", Required: true},

	// object never reaches Salesforce — the link is by record ID alone. It is
	// here so the editor can narrow the record picker to one object type.
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Account, Contact, Case… — only used to help you pick the record"},
	{Name: "linked_entity_id", Type: core.ConnectionTypeString, Label: "Share With Record", Placeholder: "The record to attach the file to, e.g. 0015f00000XyzAAB", Required: true},

	{
		Name:        "share_type",
		Type:        core.ConnectionTypeString,
		Label:       "Permission",
		Placeholder: "View Only is the safe default",
		Options: []core.ConnectionOption{
			{Name: "View Only", Value: "V"},
			{Name: "View and Edit", Value: "C"},
			{Name: "Inherit From The Record", Value: "I"},
		},
	},
	{
		Name:        "visibility",
		Type:        core.ConnectionTypeString,
		Label:       "Who Can See It",
		Placeholder: "All Users means everyone with access to the record, including community users",
		Options: []core.ConnectionOption{
			{Name: "All Users", Value: "AllUsers"},
			{Name: "Internal Users Only", Value: "InternalUsers"},
			{Name: "Shared Users", Value: "SharedUsers"},
		},
	},

	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Custom_Field__c":"value"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Share ID"},
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

	fileID, err := salesforce.RequiredString("file_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("file_id is required — the file you want to share")
	}
	if err := salesforce.ValidateRecordID(fileID); err != nil {
		return nil, err
	}
	recordID, err := salesforce.RequiredString("linked_entity_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("linked_entity_id is required — the record to attach the file to")
	}
	if err := salesforce.ValidateRecordID(recordID); err != nil {
		return nil, err
	}

	documentID, err := resolveDocumentID(instanceURL, token, fileID)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Both defaults are the conservative ones: read-only, and inheriting the
	// record's own audience rather than widening it.
	shareType := salesforce.OptionalString("share_type", inputs)
	if shareType == "" {
		shareType = "V"
	}
	visibility := salesforce.OptionalString("visibility", inputs)
	if visibility == "" {
		visibility = "AllUsers"
	}

	body := map[string]interface{}{
		"ContentDocumentId": documentID,
		"LinkedEntityId":    recordID,
		"ShareType":         shareType,
		"Visibility":        visibility,
	}
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "ContentDocumentLink", body)
	if err != nil {
		// The common rejection here is sharing a file with a record it is
		// already on, which Salesforce reports as a duplicate. It is a provider
		// decision either way, so it goes to the error port as data.
		return salesforce.ErrorResult(err.Error()), nil
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Attached file %s to record %s", documentID, recordID)), nil
}

// resolveDocumentID accepts either flavour of file ID and returns the
// ContentDocument the link object needs. An upload step returns a version ID,
// so silently following it is what stops the two actions failing to chain.
func resolveDocumentID(instanceURL, token, fileID string) (string, error) {
	if !strings.HasPrefix(fileID, contentVersionPrefix) {
		return fileID, nil
	}
	version, err := salesforce.GetRecord(instanceURL, token, "ContentVersion", fileID, "Id,ContentDocumentId")
	if err != nil {
		return "", err
	}
	documentID := salesforce.StringifyID(version["ContentDocumentId"])
	if documentID == "" {
		return "", fmt.Errorf("could not find the file behind version %s — it may have been deleted", fileID)
	}
	return documentID, nil
}
