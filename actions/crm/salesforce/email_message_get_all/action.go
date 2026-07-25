// Package crm_salesforce_email_message_get_all reads the email correspondence
// recorded against a Case or another Salesforce record.
//
// This is the read side of the Case inbox: "what has already been said to this
// customer" is the first thing anybody needs before replying, and it is the
// thing that is hardest to get at, because EmailMessage has no REST list
// endpoint of its own — it is reachable only through SOQL.
//
// Two details that would otherwise cost an afternoon:
//
//   - EmailMessage has NO Name field, so the generic default SELECT list every
//     other object uses would fail outright with INVALID_FIELD. This package
//     carries its own default field list.
//   - Newest-first is what a thread view means. Salesforce's own default order
//     is unspecified, so an explicit ORDER BY MessageDate DESC is applied
//     unless the operator asks for something else.
package crm_salesforce_email_message_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Emails"
	Description  = "Read the emails logged against a Case or another Salesforce record, newest first. Enable Return All to fetch every message in the thread."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+comments"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// defaultEmailMessageFields is the SELECT list used when the operator picks no
// fields. EmailMessage has no Name field, so the shared per-object default
// (Id, Name, LastModifiedDate) cannot be used — it would fail the whole query.
// TextBody is included deliberately: reading the conversation is the point of
// the action, and a thread with the bodies stripped out is not a thread.
const defaultEmailMessageFields = "Id, ParentId, RelatedToId, Subject, FromAddress, FromName, ToAddress, CcAddress, BccAddress, Incoming, Status, MessageDate, HasAttachment, TextBody"

// defaultOrder shows the most recent message first, which is how every mail
// client and every Case feed presents a thread.
const defaultOrder = "MessageDate DESC"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Case", Placeholder: "5005f00000AbcDEAA — read the email thread on this Case"},
	{Name: "related_to_id", Type: core.ConnectionTypeString, Label: "Related To Record", Placeholder: "0065f00000AbcDEAA — read emails linked to this Account, Opportunity or other record"},
	{
		Name:        "direction",
		Type:        core.ConnectionTypeString,
		Label:       "Direction",
		Placeholder: "All emails unless you narrow it",
		Options: []core.ConnectionOption{
			{Name: "Any", Value: "any"},
			{Name: "Received Only", Value: "incoming"},
			{Name: "Sent Only", Value: "outgoing"},
		},
	},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return (optional)"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: "[{\"field\":\"FromAddress\",\"operator\":\"=\",\"value\":\"jane@acme.com\"}]"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "MessageDate DESC (the default) — or Subject ASC"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every matching email)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 by default, up to 2000; ignored when Return All is on"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted and Archived Emails"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Emails"},
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

	// The record filters are built as ordinary conditions so they go through
	// the same validation and escaping as everything else — there is no
	// hand-assembled SOQL anywhere in this action.
	conditions := []salesforce.Condition{}

	if parentID := salesforce.OptionalString("parent_id", inputs); parentID != "" {
		if err := salesforce.ValidateRecordID(parentID); err != nil {
			return nil, fmt.Errorf("Case: %w", err)
		}
		conditions = append(conditions, salesforce.Condition{Field: "ParentId", Operator: "=", Value: parentID})
	}
	if relatedToID := salesforce.OptionalString("related_to_id", inputs); relatedToID != "" {
		if err := salesforce.ValidateRecordID(relatedToID); err != nil {
			return nil, fmt.Errorf("Related To Record: %w", err)
		}
		conditions = append(conditions, salesforce.Condition{Field: "RelatedToId", Operator: "=", Value: relatedToID})
	}
	switch salesforce.OptionalString("direction", inputs) {
	case "incoming":
		conditions = append(conditions, salesforce.Condition{Field: "Incoming", Operator: "=", Value: "true"})
	case "outgoing":
		conditions = append(conditions, salesforce.Condition{Field: "Incoming", Operator: "=", Value: "false"})
	}

	// Extra filters are appended rather than replacing the record filters, and
	// everything is joined with AND: "emails on this Case, from this address"
	// is what an operator means when they fill in both.
	extra, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, extra...)

	fields := salesforce.OptionalString("fields", inputs)
	if fields == "" {
		fields = defaultEmailMessageFields
	}
	orderBy := salesforce.OptionalString("order_by", inputs)
	if orderBy == "" {
		orderBy = defaultOrder
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	pageSize := salesforce.ClampLimit(limit, limitSet)

	// A LIMIT is only applied for a single page. With Return All on, the cap
	// would silently truncate the thread the operator asked to see in full.
	//
	// BuildQueryTyped, not BuildQuery: More Filters carries operator-supplied
	// values, and whether one is quoted depends on the FIELD's type — Incoming
	// and HasAttachment want a bare true/false, and are INVALID_FIELD quoted.
	// One cached describe settles it; if the connected user cannot describe
	// EmailMessage the builder falls back rather than failing the read.
	soql, err := salesforce.BuildQueryTyped(instanceURL, token, "EmailMessage", fields, conditions, false, orderBy, pageSize, !returnAll)
	if err != nil {
		return nil, err
	}

	includeDeleted := salesforce.OptionalBool("include_deleted", inputs)
	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, includeDeleted)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d email(s) across %d page(s); stopped at the %d-page safety cap — %d match in total", len(records), pages, salesforce.MaxAllPages, totalSize)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d email(s) across %d page(s)", len(records), pages)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d email(s) of %d matching", len(records), totalSize)
	}
	return out, nil
}
