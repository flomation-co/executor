// Package crm_salesforce_case_comment_get_all reads the comment thread on a
// Case.
//
// n8n can post a case comment but has no way to read one back, which quietly
// rules out the most useful helpdesk flow there is: "when an agent replies on a
// case, put the reply in the team's Slack channel". Without a read there is
// nothing to put anywhere.
//
// Comments live on their own object, so listing them means a SOQL query against
// CaseComment filtered by ParentId — the case. Everything the operator supplies
// goes through the shared query builder, which is the injection boundary for the
// whole node.
package crm_salesforce_case_comment_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Case Comments"
	Description  = "Read the comments on a Salesforce case, newest first. Set Limit to 1 to grab just the latest reply — the usual way to push an agent's update into Slack or Teams."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+comments"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// commentListFields is the SELECT list used when the operator picks no columns.
//
// It cannot be salesforce.DefaultFields: that helper falls back to
// "Id,Name,LastModifiedDate" for any object it does not know, and CaseComment
// has no Name field — the query would fail outright with INVALID_FIELD. These
// are the fields a thread reader needs: what was said, whether the customer can
// see it, who said it and when.
const commentListFields = "Id,ParentId,CommentBody,IsPublished,CreatedDate,CreatedById,LastModifiedDate,LastModifiedById"

// defaultCommentOrder puts the newest comment first, so a Limit of 1 returns the
// latest reply. A thread read in full is still readable either way round; a
// "latest comment" flow only works in this order.
const defaultCommentOrder = "CreatedDate DESC"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	{Name: "case_id", Type: core.ConnectionTypeString, Label: "Case ID", Placeholder: "5005f00000XyzAAAAQ — the case whose comments you want", Required: true},

	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Leave blank for the usual comment columns, or list them: CommentBody,CreatedDate"},

	// Published comments are the ones the customer can see in the portal.
	// Filtering to them is how a flow forwards the agent's public reply without
	// leaking an internal note alongside it.
	{Name: "published_only", Type: core.ConnectionTypeBoolean, Label: "Only Comments the Customer Can See"},

	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "CreatedDate DESC — newest first (the default)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every comment)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 by default, up to 2000 (ignored when Return All is on)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Comments"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Matching"},
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

	caseID, err := salesforce.RequiredString("case_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("case_id is required — the ID of the case whose comments you want")
	}
	// Without this guard a mistyped ID becomes a WHERE clause that matches
	// nothing, and the action reports "0 comments" for a case that has plenty.
	// A wrong ID is a configuration mistake, so it fails loudly instead.
	if err := salesforce.ValidateRecordID(caseID); err != nil {
		return nil, err
	}

	// ParentId is the link from a comment back to its case; it is always the
	// first condition, and the operator cannot remove it.
	conditions := []salesforce.Condition{{Field: "ParentId", Operator: "=", Value: caseID}}
	if salesforce.OptionalBool("published_only", inputs) {
		conditions = append(conditions, salesforce.Condition{Field: "IsPublished", Operator: "=", Value: "true"})
	}

	fields := salesforce.OptionalString("fields", inputs)
	if fields == "" {
		fields = commentListFields
	}
	orderBy := salesforce.OptionalString("order_by", inputs)
	if orderBy == "" {
		orderBy = defaultCommentOrder
	}
	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// The LIMIT clause is what caps a single page, so it must NOT be applied
	// when the operator asked for every comment.
	soql, err := salesforce.BuildQuery("CaseComment", fields, conditions, false, orderBy, salesforce.ClampLimit(limit, limitSet), !returnAll)
	if err != nil {
		// An unusable field name or sort direction is a configuration mistake.
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, false)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d comment(s) on case %s across %d page(s); stopped at the %d-page safety cap", len(records), caseID, pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d comment(s) on case %s", len(records), caseID)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d comment(s) on case %s of %d in total", len(records), caseID, totalSize)
	}
	return out, nil
}
