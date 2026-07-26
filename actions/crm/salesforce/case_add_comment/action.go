// Package crm_salesforce_case_add_comment posts a comment onto a Case.
//
// A case comment is a separate record, not a field on the case: it is written to
// the CaseComment object with the case as its parent. That is worth knowing
// because it is why this is its own action rather than an option on the update,
// and why deleting a comment (case_comment_delete) takes the comment's ID and
// not the case's.
//
// Two differences from n8n, both deliberate. The comment body is required here —
// n8n leaves it optional, so an empty comment can be posted, which is never what
// anyone meant. And the 4,000-BYTE limit is checked before the call: it is bytes
// rather than characters, so accented or non-Latin text runs out of room sooner
// than the operator expects and the server-side rejection does not say why.
package crm_salesforce_case_add_comment

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Case Comment"
	Description  = "Post a comment onto a Salesforce case — an internal note for the team, or a reply the customer can see in the portal."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+comment"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// maxCommentBytes is Salesforce's cap on CommentBody. It is measured in BYTES,
// not characters — 4,000 plain ASCII characters fit, but the same field holds
// only around 1,333 emoji. Checking locally turns a bare STRING_TOO_LONG into a
// message that says which limit was hit and by how much.
const maxCommentBytes = 4000

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	{Name: "case_id", Type: core.ConnectionTypeString, Label: "Case ID", Placeholder: "5005f00000XyzAAAAQ — the case to comment on", Required: true},
	{Name: "comment_body", Type: core.ConnectionTypeText, Label: "Comment", Placeholder: "Called the customer back — replacement part ordered, due Friday", Required: true},

	// Off by default, and that default matters: an internal note that gets
	// published is visible to the customer in the Self-Service portal, and there
	// is no unsee. Turning it on is the deliberate act.
	{Name: "is_published", Type: core.ConnectionTypeBoolean, Label: "Make Visible to the Customer"},

	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"Custom_Field__c":"value"} — extra CaseComment fields`},
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

	caseID, err := salesforce.RequiredString("case_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("case_id is required — the ID of the case to comment on")
	}
	if err := salesforce.ValidateRecordID(caseID); err != nil {
		return nil, err
	}

	comment, err := salesforce.RequiredString("comment_body", inputs)
	if err != nil {
		return nil, fmt.Errorf("comment_body is required — there is no point posting an empty comment")
	}
	if len(comment) > maxCommentBytes {
		return nil, fmt.Errorf("comment_body is %d bytes; Salesforce allows %d. Note the limit is in bytes, so accented or non-Latin characters use more than one each — shorten the comment or split it across two", len(comment), maxCommentBytes)
	}

	// ParentId is what attaches the comment to the case; the comment itself
	// carries no case field of any other name.
	body := map[string]interface{}{
		"ParentId":    caseID,
		"CommentBody": comment,
	}
	salesforce.SetBoolIfSet(body, inputs, "IsPublished", "is_published")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "CaseComment", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	visibility := "internal only"
	if salesforce.OptionalBool("is_published", inputs) {
		visibility = "visible to the customer"
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Added a comment to case %s (%s)", caseID, visibility)), nil
}
