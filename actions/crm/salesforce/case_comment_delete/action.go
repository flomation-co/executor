// Package crm_salesforce_case_comment_delete removes a comment from a Case.
//
// The counterpart to case_add_comment, and the reason it exists is automation
// rather than housekeeping: a flow that posts case updates will eventually post
// the wrong one — a draft, a duplicate from a retried webhook, or a note that
// was published to the customer portal by mistake. n8n has no way to take one
// back, so the only remedy there is a person in Salesforce doing it by hand.
//
// It takes the COMMENT's ID, not the case's. Use case_comment_get_all to find
// it; the comment ID starts 00a, while the case ID starts 500.
package crm_salesforce_case_comment_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Case Comment"
	Description  = "Remove a comment from a Salesforce case — for a note posted in error, or one published to the customer portal by mistake. Give it the comment's ID, not the case's."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+xmark"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID", Placeholder: "00a5f00000XyzAAAAQ — the comment, not the case", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Comment ID"},
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

	commentID, err := salesforce.RequiredString("comment_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("comment_id is required — the ID of the comment to delete, which you can get from Get Many Case Comments")
	}
	// A malformed ID is a configuration mistake, so it fails hard here rather
	// than travelling to Salesforce and coming back as MALFORMED_ID. Passing the
	// CASE id by mistake is the likely slip, and that is a well-formed ID, so it
	// gets through to Salesforce and comes back as a wrong-object error — which
	// is why the input label spells out which ID this wants.
	if err := salesforce.ValidateRecordID(commentID); err != nil {
		return nil, err
	}

	if err := salesforce.DeleteRecord(instanceURL, token, "CaseComment", commentID); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// DELETE answers 204 No Content, so the ID we were given is the only thing
	// there is to hand on.
	result := map[string]interface{}{"Id": commentID, "deleted": true}
	return salesforce.RecordResult(commentID, result, fmt.Sprintf("Deleted case comment %s", commentID)), nil
}
