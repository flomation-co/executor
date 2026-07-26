// Package crm_salesforce_file_get_all_for_record lists the files attached to a
// record — the Lightning "Files" related list, read from a flow.
//
// n8n cannot answer "what is attached to this account?" at all, which blocks
// the obvious follow-on work: forward every file on a case to the engineer,
// check a signed contract arrived, archive a customer's documents on closure.
//
// The join object is ContentDocumentLink, and it has two rules that shape this
// action. A query MUST filter on LinkedEntityId or ContentDocumentId — an
// unfiltered SELECT is rejected outright — and it does not page or sort like an
// ordinary object, which is why this returns a single bounded page rather than
// offering Return All.
package crm_salesforce_file_get_all_for_record

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Files On A Record"
	Description  = "List the files attached to any Salesforce record, with their names, sizes and IDs — ready to download or forward in a later step."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// linkFields is the SELECT list. It reaches through the ContentDocument
// relationship for everything a human needs (title, extension, size) and
// carries LatestPublishedVersionId so the download action can be wired straight
// on without another lookup.
const linkFields = "Id,ContentDocumentId,LinkedEntityId,ShareType,Visibility,ContentDocument.Title,ContentDocument.FileExtension,ContentDocument.FileType,ContentDocument.ContentSize,ContentDocument.LatestPublishedVersionId,ContentDocument.OwnerId,ContentDocument.CreatedDate,ContentDocument.LastModifiedDate"

// noteFileType is the ContentDocument file type Enhanced Notes are stored as.
// Notes live in the same store as files, so without this filter every note on
// the record turns up in a list of "files" and confuses the operator.
const noteFileType = "SNOTE"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	// object never reaches Salesforce — the link is by record ID alone. It is
	// here so the editor can narrow the record picker to one object type.
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Account, Contact, Case… — only used to help you pick the record"},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record", Placeholder: "The record whose files you want, e.g. 0015f00000XyzAAB", Required: true},

	{Name: "include_notes", Type: core.ConnectionTypeBoolean, Label: "Include Notes As Well As Files"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Files"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Records Returned"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
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

	recordID, err := salesforce.RequiredString("record_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("record_id is required — the record whose files you want to list")
	}
	if err := salesforce.ValidateRecordID(recordID); err != nil {
		return nil, err
	}

	// Both conditions go through the shared builder rather than being pasted
	// into a string: the record ID is operator-supplied and this is the
	// injection boundary for the whole node.
	conditions := []salesforce.Condition{
		{Field: "LinkedEntityId", Operator: "=", Value: recordID},
	}
	if !salesforce.OptionalBool("include_notes", inputs) {
		conditions = append(conditions, salesforce.Condition{Field: "ContentDocument.FileType", Operator: "!=", Value: noteFileType})
	}

	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	// ContentDocumentLink does not accept ORDER BY, so the sort argument stays
	// empty and the LIMIT is always applied — there is no Return All here.
	soql, err := salesforce.BuildQuery("ContentDocumentLink", linkFields, conditions, false, "", salesforce.ClampLimit(limit, limitSet), true)
	if err != nil {
		return nil, err
	}

	records, nextURL, totalSize, _, err := salesforce.Query(instanceURL, token, soql, false, false)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Found %d file(s) on %s", len(records), recordID)
	if len(records) == 0 {
		summary = fmt.Sprintf("No files are attached to %s", recordID)
	}
	return salesforce.ListResult(records, nextURL, totalSize, summary), nil
}
