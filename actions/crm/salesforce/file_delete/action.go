// Package crm_salesforce_file_delete removes a file from Salesforce Files.
//
// Deleting the ContentDocument is deliberately the only option offered. It
// takes every version of the file and every record it was shared with along
// with it, which is what an operator means by "delete this file" — deleting a
// single ContentVersion would leave the file in place looking untouched, and
// deleting one ContentDocumentLink only unshares it from one record.
//
// The document goes to the Recycle Bin, so this is recoverable for 15 days.
package crm_salesforce_file_delete

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete File"
	Description  = "Delete a file from Salesforce Files, including every version of it and every record it was shared with. It goes to the Recycle Bin for 15 days."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// contentVersionPrefix is the Salesforce key prefix for a ContentVersion (one
// revision of a file). Deleting works on the ContentDocument (069…), so a
// version ID — which is what an upload step hands back — is followed up first.
const contentVersionPrefix = "068"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File", Placeholder: "File ID (069…) or version ID (068…) from an upload or files list", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "File ID"},
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
		return nil, fmt.Errorf("file_id is required — the file to delete")
	}
	if err := salesforce.ValidateRecordID(fileID); err != nil {
		return nil, err
	}

	documentID, err := resolveDocumentID(instanceURL, token, fileID)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	if err := salesforce.DeleteRecord(instanceURL, token, "ContentDocument", documentID); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers a delete with 204 No Content, so the ID we already
	// hold is the whole result — returning an empty map would break every
	// downstream step that wants to log or confirm what was removed.
	return salesforce.RecordResult(documentID, map[string]interface{}{"Id": documentID, "deleted": true},
		fmt.Sprintf("Deleted file %s — it is in the Recycle Bin for 15 days", documentID)), nil
}

// resolveDocumentID accepts either flavour of file ID and returns the
// ContentDocument. An upload step returns a version ID, so following it is what
// stops "upload then delete on failure" from silently doing nothing useful.
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
		return "", fmt.Errorf("could not find the file behind version %s — it may already have been deleted", fileID)
	}
	return documentID, nil
}
